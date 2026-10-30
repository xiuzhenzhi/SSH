package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sshguard/internal/geoip"
	"sshguard/internal/storage"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

func TestServerPublicAddresses(t *testing.T) {
	for _, tc := range []struct {
		raw, observed string
		want          []string
	}{
		{`["172.17.0.1","172.18.0.1","172.19.0.1","9.9.9.9"]`, "", []string{"9.9.9.9"}},
		{`["10.0.0.2"]`, "8.8.8.8", []string{"8.8.8.8"}},
		{`["10.0.0.2"]`, "10.1.1.1", []string{}},
		{`["2606:4700:4700::1111","8.8.8.8","8.8.8.8"]`, "9.9.9.9", []string{"8.8.8.8", "2606:4700:4700::1111"}},
		{`["2606:4700:4700::1111"]`, "8.8.8.8", []string{"8.8.8.8", "2606:4700:4700::1111"}},
		{`bad-json`, "", []string{}},
	} {
		row := map[string]any{"ip_addresses": tc.raw, "observed_public_ip": tc.observed}
		if got := serverAddresses(row); !reflect.DeepEqual(got, tc.want) {
			t.Fatal(tc, got)
		}
		var legacy []string
		if err := json.Unmarshal([]byte(row["ip_addresses"].(string)), &legacy); err != nil || !reflect.DeepEqual(legacy, tc.want) {
			t.Fatal("legacy API exposed unfiltered addresses", row, err)
		}
		if _, found := row["observed_public_ip"]; found {
			t.Fatal("internal connection metadata exposed", row)
		}
	}
}

func TestGeoIPWaiterHonorsCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	app := &App{geo: geoIPState{client: &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"country_code":"NL","country":"Netherlands"}`))}, nil
	})}}}
	ownerDone := make(chan error, 1)
	go func() {
		_, err := app.lookupGeoIP(context.Background(), geoip.Config{Provider: "geojs"}, "8.8.8.8", "", true)
		ownerDone <- err
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	waiterDone := make(chan error, 1)
	go func() {
		_, err := app.lookupGeoIP(ctx, geoip.Config{Provider: "geojs"}, "8.8.8.8", "", true)
		waiterDone <- err
	}()
	select {
	case err := <-waiterDone:
		if err != context.DeadlineExceeded {
			t.Errorf("cancelled waiter: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("cancelled list request waited for another client's GeoIP lookup")
	}
	once.Do(func() { close(release) })
	if err := <-ownerDone; err != nil {
		t.Fatal("waiter cancelled the owner's lookup", err)
	}
}

func TestGeoIPCancelledOwnerDoesNotCancelSharedLookup(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int64
	var once sync.Once
	defer once.Do(func() { close(release) })
	app := &App{geo: geoIPState{client: &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		if requests.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"country_code":"NL"}`))}, nil
	})}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ownerDone := make(chan error, 1)
	go func() {
		_, err := app.lookupGeoIP(ctx, geoip.Config{Provider: "geojs"}, "8.8.8.8", "", true)
		ownerDone <- err
	}()
	<-started
	cancel()
	select {
	case err := <-ownerDone:
		if err != context.Canceled {
			t.Fatal("cancelled owner did not return promptly", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner request ignored cancellation")
	}
	once.Do(func() { close(release) })
	location, err := app.lookupGeoIP(context.Background(), geoip.Config{Provider: "geojs"}, "8.8.8.8", "", true)
	if err != nil || location.Code != "NL" || requests.Load() != 1 {
		t.Fatal("cancelled owner poisoned another caller or triggered a duplicate lookup", location, err, requests.Load())
	}
}

func testGeoIPSettings(t *testing.T, db *storage.DB, cfg Config, session string) {
	t.Helper()
	app := NewApp(db, cfg)
	requests := 0
	app.geo.client = &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"country_code":"NL","country":"Netherlands"}`))}, nil
	})}
	router := app.Router()
	call := func(method, path, body string, want int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, "https://guard.example.com/api/v1"+path, strings.NewReader(body))
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
		if value["data"] == nil {
			return nil
		}
		return value["data"].(map[string]any)
	}
	call("PATCH", "/settings/geoip", `{"provider":"invalid"}`, 400)
	call("PATCH", "/settings/geoip", `{"provider":"ipinfo","token":"secret-fixture"}`, 200)
	value := call("GET", "/settings/geoip", "", 200)
	if value["token"] != nil || value["token_configured"] != true {
		t.Fatal("token exposed or not saved", value)
	}
	call("PATCH", "/settings/geoip", `{"provider":"ipinfo"}`, 200)
	stored, err := app.readGeoIP(context.Background())
	if err != nil || stored.Token != "secret-fixture" {
		t.Fatal("token not retained", err)
	}
	call("PATCH", "/settings/geoip", `{"provider":"geojs"}`, 200)
	stored, err = app.readGeoIP(context.Background())
	if err != nil || stored.Token != "" {
		t.Fatal("token leaked between providers", err)
	}
	call("POST", "/settings/geoip/test", `{"ip":"10.0.0.2"}`, 400)
	value = call("POST", "/settings/geoip/test", `{"ip":"8.8.8.8"}`, 200)
	if value["country_code"] != "NL" || requests != 1 {
		t.Fatal("GeoIP test did not use saved provider", value, requests)
	}
	row := map[string]any{"ip_addresses": `["172.17.0.1","8.8.8.8"]`}
	app.decorateServers(context.Background(), []map[string]any{row})
	if row["country_code"] != "NL" || requests != 1 {
		t.Fatal("lookup not cached", row, requests)
	}
	call("PATCH", "/settings/geoip", `{"provider":"none"}`, 200)
	app.decorateServers(context.Background(), []map[string]any{row})
	if row["country_code"] != "" || requests != 1 {
		t.Fatal("disabled provider still used")
	}
	call("POST", "/settings/geoip/test", `{"ip":"8.8.8.8"}`, 400)
	call("PATCH", "/settings/geoip", `{"provider":"maxmind"}`, 200)
	call("POST", "/settings/geoip/test", `{"ip":"8.8.8.8"}`, 409)
	// A malformed update must leave an existing saved database untouched.
	if _, err := db.Exec(context.Background(), `INSERT INTO geoip_database(id,revision,content,updated_at) VALUES(1,?,?,CURRENT_TIMESTAMP)`, uuid.NewString(), []byte("old fixture")); err != nil {
		t.Fatal(err)
	}
	call("POST", "/settings/geoip/update", `{}`, 502)
	var content []byte
	if err := db.Row(context.Background(), `SELECT content FROM geoip_database WHERE id=1`).Scan(&content); err != nil || string(content) != "old fixture" {
		t.Fatal("failed update replaced database", err)
	}
	db.Exec(context.Background(), `DELETE FROM geoip_database WHERE id=1`)
	call("PATCH", "/settings/geoip", `{"provider":"none"}`, 200)
	// Both transports supply this trusted source internally, outside agent JSON.
	id, credential := uuid.NewString(), token()
	if _, err := db.Exec(context.Background(), `INSERT INTO servers(id,name) VALUES(?,?)`, id, "NAT fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `INSERT INTO agent_credentials(token_hash,server_id) VALUES(?,?)`, hash(credential), id); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "https://guard.example.com/api/v1/agent/heartbeat", strings.NewReader(`{"report_interval_seconds":5,"ip_addresses":["10.0.0.2"],"source_ip":"9.9.9.9"}`))
	req.RemoteAddr = "8.8.8.8:12345"
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatal("NAT heartbeat failed", w.Body.String())
	}
	value = call("GET", "/servers/"+id, "", 200)
	if !reflect.DeepEqual(value["public_ip_addresses"], []any{"8.8.8.8"}) {
		t.Fatal("NAT fallback missing", value)
	}
	// The WebSocket path also uses the source derived from trusted proxies.
	app.cfg.TrustedProxyCIDRs = []string{"127.0.0.1/32"}
	streamServer := httptest.NewServer(app.Router())
	defer streamServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(streamServer.URL, "http")+"/api/v1/agent/stream", &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + credential}, "X-Forwarded-For": []string{"8.8.4.4"}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if err := wsjson.Write(ctx, conn, map[string]any{"request_id": uuid.NewString(), "type": "heartbeat", "data": map[string]any{"source_ip": "9.9.9.9"}}); err != nil {
		t.Fatal(err)
	}
	var ack agentAck
	if err := wsjson.Read(ctx, conn, &ack); err != nil || ack.Status != 200 {
		t.Fatal("stream heartbeat failed", ack, err)
	}
	value = call("GET", "/servers/"+id, "", 200)
	if !reflect.DeepEqual(value["public_ip_addresses"], []any{"8.8.4.4"}) {
		t.Fatal("WebSocket NAT fallback missing", value)
	}
	// Keep subsequent integration checks independent of this fixture's config.
	call("PATCH", "/settings/geoip", `{"provider":"none"}`, 200)
}
