package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sshguard/internal/storage"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEventCountriesAndBatchAcknowledgement(t *testing.T) {
	ctx := context.Background()
	driver, dsn := os.Getenv("TEST_FEATURE_DB_DRIVER"), os.Getenv("TEST_FEATURE_DATABASE_URL")
	if dsn == "" {
		driver, dsn = "sqlite", filepath.Join(t.TempDir(), "events.db")
	}
	db, err := storage.Open(ctx, driver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Driver: driver, PublicURL: "https://guard.example.com", AdminUsername: "admin", AdminPassword: "map-test-password-2026", EncryptionKey: strings.Repeat("ab", 32)}
	if err = Bootstrap(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	app := NewApp(db, cfg)
	var lookups atomic.Int64
	app.geo.client = &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		lookups.Add(1)
		code := "US"
		if strings.Contains(r.URL.Path, "9.9.9.9") {
			code = "DE"
		}
		if strings.Contains(r.URL.Path, "2606:") {
			code = "CN"
		}
		for address, value := range map[string]string{"9.9.9.10": "NL", "9.9.9.11": "FR", "9.9.9.12": "GB", "9.9.9.13": "JP"} {
			if strings.Contains(r.URL.Path, address) {
				code = value
			}
		}
		if strings.Contains(r.URL.Path, "8.8.4.4") {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unavailable"))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"country_code":"` + code + `"}`))}, nil
	})}
	router := app.Router()
	call := func(method, path, body, session string, want int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, cfg.PublicURL+"/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Cookie", session)
		req.Header.Set("Origin", cfg.PublicURL)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		var value map[string]any
		json.Unmarshal(w.Body.Bytes(), &value)
		data, _ := value["data"].(map[string]any)
		return data
	}
	login := func(name string) string {
		t.Helper()
		req := httptest.NewRequest("POST", cfg.PublicURL+"/api/v1/auth/login", strings.NewReader(`{"username":"`+name+`","password":"map-test-password-2026"}`))
		req.Header.Set("Origin", cfg.PublicURL)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		cookie := w.Result().Cookies()[0]
		return cookie.Name + "=" + cookie.Value
	}
	session := login("admin")
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	serverID := uuid.NewString()
	exec(`INSERT INTO servers(id,name) VALUES(?,?)`, serverID, "Map fixture")
	now := time.Now().UTC()
	for _, event := range []struct {
		ip, outcome string
		at          time.Time
	}{
		{"8.8.8.8", "failure", now}, {"8.8.8.8", "failure", now}, {"1.1.1.1", "failure", now},
		{"9.9.9.9", "failure", now}, {"2606:4700:4700::1111", "failure", now},
		{"10.0.0.1", "failure", now}, {"192.0.2.1", "failure", now}, {"8.8.4.4", "failure", now},
		{"8.8.8.8", "success", now}, {"8.8.8.8", "failure", now.Add(-25 * time.Hour)}, {"8.8.8.8", "failure", now.Add(time.Hour)},
	} {
		kind := "ssh.login_failed"
		if event.outcome == "success" {
			kind = "ssh.login"
		}
		exec(`INSERT INTO ssh_events(event_id,server_id,event_type,username,source_ip,outcome,occurred_at) VALUES(?,?,?,?,?,?,?)`, uuid.NewString(), serverID, kind, "root", event.ip, event.outcome, event.at)
	}
	events := call("GET", "/events?source_ip=9.9.9.9", "", session, 200)["items"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["country_code"] != "DE" || lookups.Load() != 1 {
		t.Fatal("event country must use source IP and the shared location cache", events, lookups.Load())
	}
	var eventID int64
	if err := db.Row(ctx, `SELECT id FROM ssh_events WHERE source_ip='9.9.9.9'`).Scan(&eventID); err != nil {
		t.Fatal(err)
	}
	if call("GET", "/events/"+strconv.FormatInt(eventID, 10), "", session, 200)["country_code"] != "DE" {
		t.Fatal("event detail is missing its country")
	}
	if lookups.Load() != 1 {
		t.Fatal("event detail did not reuse cached GeoIP")
	}
	for _, address := range []string{"8.8.8.8", "2606:4700:4700::1111", "10.0.0.1", "192.0.2.1", "8.8.4.4"} {
		items := call("GET", "/events?source_ip="+address, "", session, 200)["items"].([]any)
		for _, item := range items {
			row := item.(map[string]any)
			code := ""
			if address == "8.8.8.8" {
				code = "US"
			}
			if strings.Contains(address, "2606:") {
				code = "CN"
			}
			if row["country_code"] != code {
				t.Fatal("unexpected source country", row)
			}
		}
	}
	if lookups.Load() != 4 {
		t.Fatal("duplicate IPs or reserved addresses triggered extra lookups", lookups.Load())
	}
	call("GET", "/dashboard/country-ranking", "", "", 401)
	ranking := call("GET", "/dashboard/country-ranking", "", session, 200)
	if ranking["total"] != float64(8) || ranking["unlocated"] != float64(3) || ranking["country_count"] != float64(3) {
		t.Fatal("country totals must exclude successes, old and future events, and keep unknown counts", ranking)
	}
	if top := ranking["items"].([]any)[0].(map[string]any); top["country_code"] != "US" || top["count"] != float64(3) {
		t.Fatal("multiple IPs from the same country were not aggregated", top)
	}
	for address, count := range map[string]int{"9.9.9.10": 3, "9.9.9.11": 2, "9.9.9.12": 4, "9.9.9.13": 1} {
		for i := 0; i < count; i++ {
			exec(`INSERT INTO ssh_events(event_id,server_id,event_type,username,source_ip,outcome,occurred_at) VALUES(?,?,'ssh.login_failed','root',?,'failure',?)`, uuid.NewString(), serverID, address, now)
		}
	}
	app.dashboardMu.Lock()
	delete(app.dashboardCache, "country-ranking")
	app.dashboardMu.Unlock()
	ranking = call("GET", "/dashboard/country-ranking", "", session, 200)
	topCountries := ranking["items"].([]any)
	if len(topCountries) != 5 || ranking["country_count"] != float64(7) || ranking["total"] != float64(18) {
		t.Fatal("country ranking must return only the top five while retaining full totals", ranking)
	}
	for i, code := range []string{"GB", "NL", "US", "FR", "CN"} {
		if topCountries[i].(map[string]any)["country_code"] != code {
			t.Fatal("ranking must sort by count with a stable country-code tie break", topCountries)
		}
	}
	lookupCount := lookups.Load()
	call("GET", "/dashboard/country-ranking", "", session, 200)
	if lookups.Load() != lookupCount {
		t.Fatal("cached ranking performed extra lookups")
	}
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for _, id := range ids {
		exec(`INSERT INTO alerts(id,severity,title,message) VALUES(?,'high','fixture','fixture')`, id)
	}
	resolved := uuid.NewString()
	exec(`INSERT INTO alerts(id,severity,title,message,resolved_at) VALUES(?,'high','resolved','fixture',?)`, resolved, now)
	call("POST", "/alerts/acknowledge", `{"all":true}`, "", 401)
	for _, origin := range []string{"", "https://untrusted.example"} {
		req := httptest.NewRequest("POST", cfg.PublicURL+"/api/v1/alerts/acknowledge", strings.NewReader(`{"all":true}`))
		req.Header.Set("Cookie", session)
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("batch acknowledgement accepted a missing or forged Origin", w.Code)
		}
	}
	oversized := `{"ids":[` + strings.TrimSuffix(strings.Repeat(`"`+ids[0]+`",`, 1001), ",") + `]}`
	call("POST", "/alerts/acknowledge", oversized, session, 400)
	call("POST", "/alerts/acknowledge", `{"ids":["' OR 1=1 --"]}`, session, 400)
	call("POST", "/alerts/acknowledge", `{}`, session, 400)
	call("POST", "/alerts/acknowledge", `{"ids":["invalid"]}`, session, 400)
	call("POST", "/alerts/acknowledge", `{"all":true,"ids":["`+ids[0]+`"]}`, session, 400)
	call("POST", "/alerts/acknowledge", `{"ids":["`+ids[0]+`","`+uuid.NewString()+`"]}`, session, 404)
	var pending int
	if err := db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE acknowledged_at IS NULL`).Scan(&pending); err != nil || pending != 5 {
		t.Fatal("invalid selection partially committed", pending, err)
	}
	data := call("POST", "/alerts/acknowledge", `{"ids":["`+ids[0]+`","`+strings.ToUpper(ids[0])+`","`+resolved+`"]}`, session, 200)
	if data["acknowledged"] != float64(1) {
		t.Fatal("batch did not deduplicate IDs or skip resolved alerts", data)
	}
	if call("POST", "/alerts/acknowledge", `{"ids":["`+ids[0]+`"]}`, session, 200)["acknowledged"] != float64(0) {
		t.Fatal("an already acknowledged selection must be a no-op")
	}
	for _, role := range []string{"viewer", "operator"} {
		call("POST", "/users", `{"username":"`+role+`","password":"map-test-password-2026","role":"`+role+`"}`, session, 200)
		cookie := login(role)
		want := 403
		if role == "operator" {
			want = 200
		}
		call("POST", "/alerts/acknowledge", `{"ids":["`+ids[1]+`"]}`, cookie, want)
		call("GET", "/events?source_ip=9.9.9.9", "", cookie, 200)
		call("GET", "/dashboard/country-ranking", "", cookie, 200)
	}
	data = call("POST", "/alerts/acknowledge", `{"all":true}`, session, 200)
	if data["acknowledged"] != float64(2) {
		t.Fatal("all-pending acknowledgement missed records", data)
	}
	if call("POST", "/alerts/acknowledge", `{"all":true}`, session, 200)["acknowledged"] != float64(0) {
		t.Fatal("repeated batch confirmation must be idempotent")
	}
	var actor string
	if err := db.Row(ctx, `SELECT u.username FROM alerts a JOIN users u ON a.acknowledged_by=u.id WHERE a.id=?`, ids[1]).Scan(&actor); err != nil || actor != "operator" {
		t.Fatal("batch acknowledgement did not preserve the actor", actor, err)
	}
	call("PATCH", "/settings/geoip", `{"provider":"none"}`, session, 200)
	events = call("GET", "/events?source_ip=9.9.9.9", "", session, 200)["items"].([]any)
	if len(events) != 1 || events[0].(map[string]any)["country_code"] != "" || lookups.Load() != lookupCount {
		t.Fatal("disabled GeoIP must not perform lookups or use stale country data", events)
	}
	ranking = call("GET", "/dashboard/country-ranking", "", session, 200)
	if ranking["geoip_status"] != "disabled" || len(ranking["items"].([]any)) != 0 || ranking["unlocated"] != float64(18) {
		t.Fatal("disabled GeoIP must immediately invalidate the previous ranking", ranking)
	}
	// A lookup started before settings change must not repopulate stale data.
	call("PATCH", "/settings/geoip", `{"provider":"geojs"}`, session, 200)
	app.geo.mu.Lock()
	app.geo.cache = nil
	app.geo.mu.Unlock()
	started, release := make(chan struct{}), make(chan struct{})
	var once, finish sync.Once
	defer finish.Do(func() { close(release) })
	app.geo.client.Transport = captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"country_code":"US"}`))}, nil
	})
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest("GET", cfg.PublicURL+"/api/v1/dashboard/country-ranking", nil)
		req.Header.Set("Cookie", session)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		done <- w
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("ranking did not start the lookup")
	}
	call("PATCH", "/settings/geoip", `{"provider":"none"}`, session, 200)
	finish.Do(func() { close(release) })
	select {
	case response := <-done:
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("old ranking request did not complete")
	}
	ranking = call("GET", "/dashboard/country-ranking", "", session, 200)
	if ranking["geoip_status"] != "disabled" || len(ranking["items"].([]any)) != 0 {
		t.Fatal("in-flight lookup overwrote the disabled GeoIP cache", ranking)
	}
}
