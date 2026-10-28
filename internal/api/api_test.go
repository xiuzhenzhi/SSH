package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"os"
	"sshguard/internal/storage"
	"sshguard/migrations"
	"strings"
	"testing"
	"time"
)

func TestValidEvent(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name, kind, outcome string
		at                  time.Time
		want                bool
	}{{"success", "ssh.login", "success", now, true}, {"mismatch", "ssh.login", "failure", now, false}, {"old", "ssh.login", "success", now.AddDate(0, 0, -31), false}, {"future", "ssh.login", "success", now.Add(6 * time.Minute), false}, {"failure", "ssh.login_failed", "failure", now, true}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validEvent(Event{Type: tc.kind, Outcome: tc.outcome, At: tc.at}, now); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
func TestConfigRequiresHTTPS(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("DATABASE_URL", "test")
	for _, origin := range []string{"http://guard.example.com", "https://guard.example.com/path", "https://user:pass@guard.example.com"} {
		t.Setenv("PUBLIC_URL", origin)
		if _, e := LoadConfig(); e == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	t.Setenv("PUBLIC_URL", "http://localhost:8088")
	if c, e := LoadConfig(); e != nil || c.SecureCookie {
		t.Fatal("localhost mode invalid", e)
	}
}

// TEST_DATABASE_URL must point to an isolated, empty test database. No drop/truncate is performed.
func TestIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL and TEST_DB_DRIVER for real database integration")
	}
	driver := os.Getenv("TEST_DB_DRIVER")
	if driver == "" {
		driver = "postgres"
	}
	ctx := context.Background()
	db, e := storage.Open(ctx, driver, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if driver == "mysql" && os.Getenv("SSHGUARD_TEST_SCHEMA") == "legacy" {
		// Reproduce old deployments whose foreign-key UUID columns use utf8mb4_bin.
		for _, name := range []string{"001_init.sql", "002_events.sql"} {
			data, e := migrations.Files.ReadFile("mysql/" + name)
			if e != nil {
				t.Fatal(e)
			}
			for _, query := range strings.Split(strings.ReplaceAll(string(data), "utf8mb4_unicode_ci", "utf8mb4_bin"), ";") {
				if strings.TrimSpace(query) == "" {
					continue
				}
				if _, e = db.Exec(ctx, query); e != nil {
					t.Fatal("legacy schema fixture", e)
				}
			}
		}
	}
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal("migration repeat", e)
	}
	if _, e = db.Exec(ctx, `DELETE FROM schema_migrations WHERE version=?`, "003_maintenance_indexes.sql"); e != nil {
		t.Fatal(e)
	}
	if e = storage.Migrate(ctx, db); e != nil {
		t.Fatal("interrupted index migration retry", e)
	}
	if driver == "mysql" {
		var zone, charset, version string
		if e = db.Row(ctx, `SELECT @@session.time_zone,@@character_set_connection,VERSION()`).Scan(&zone, &charset, &version); e != nil || zone != "+00:00" || charset != "utf8mb4" {
			t.Fatal("MySQL session settings", zone, charset, e)
		}
		t.Log("MySQL tested:", version)
		if os.Getenv("SSHGUARD_TEST_SCHEMA") == "legacy" {
			var collation string
			if e = db.Row(ctx, `SELECT COLLATION_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='servers' AND COLUMN_NAME='id'`).Scan(&collation); e != nil || collation != "utf8mb4_bin" {
				t.Fatal("legacy fixture collation not preserved", collation, e)
			}
		}
	}
	cfg := Config{Driver: driver, PublicURL: "https://guard.example.com", AdminUsername: "admin", AdminPassword: "integration-password-2026", SecureCookie: true, EncryptionKey: strings.Repeat("ab", 32)}
	if e = Bootstrap(ctx, db, cfg); e != nil {
		t.Fatal(e)
	}
	handler := New(db, cfg)
	call := func(method, path string, body any, cookie, agent string, origin bool, want int) (map[string]any, *httptest.ResponseRecorder) {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, "http://localhost/api/v1"+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if origin {
			req.Header.Set("Origin", cfg.PublicURL)
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		if agent != "" {
			req.Header.Set("Authorization", "Bearer "+agent)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		var value map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &value)
		return value, rec
	}
	call("GET", "/servers", nil, "", "", false, 401)
	call("POST", "/auth/login", map[string]string{"username": "admin", "password": cfg.AdminPassword}, "", "", false, 403)
	_, res := call("POST", "/auth/login", map[string]string{"username": "admin", "password": cfg.AdminPassword}, "", "", true, 200)
	cookie := res.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("insecure cookie")
	}
	session := cookie.Name + "=" + cookie.Value
	result, _ := call("POST", "/servers", map[string]string{"name": "集成测试🔒", "group_name": "生产"}, session, "", true, 200)
	id := result["data"].(map[string]any)["id"].(string)
	result, _ = call("GET", "/servers/"+id, nil, session, "", false, 200)
	if result["data"].(map[string]any)["name"] != "集成测试🔒" {
		t.Fatal("utf8mb4 round-trip failed")
	}
	call("GET", "/agent/update-commands", nil, session, "", false, 200)
	result, _ = call("POST", "/servers/"+id+"/enrollment", map[string]string{}, session, "", true, 200)
	command := result["data"].(map[string]any)["command"].(string)
	commands := result["data"].(map[string]any)["commands"].(map[string]any)
	for _, platform := range []string{"windows", "ubuntu", "debian", "alpine"} {
		if value, ok := commands[platform].(string); !ok || value == "" {
			t.Fatalf("missing %s installation command", platform)
		}
	}
	parts := strings.Split(command, "'")
	enrollment := parts[len(parts)-2]
	result, _ = call("POST", "/agent/enroll", map[string]string{"token": enrollment, "hostname": "test-host", "version": "test"}, "", "", false, 200)
	agent := result["data"].(map[string]any)["token"].(string)
	call("POST", "/agent/enroll", map[string]string{"token": enrollment, "hostname": "test-host", "version": "test"}, "", "", false, 401)
	call("POST", "/agent/heartbeat", map[string]string{}, "", agent, false, 200)
	call("POST", "/agent/heartbeat", map[string]string{"version": "upgrade-test"}, "", agent, false, 200)
	result, _ = call("GET", "/servers/"+id, nil, session, "", false, 200)
	if result["data"].(map[string]any)["agent_version"] != "upgrade-test" {
		t.Fatal("heartbeat did not update version")
	}
	event := map[string]any{"event_id": uuid.NewString(), "event_type": "ssh.login", "username": "root", "source_ip": "2001:db8::1", "outcome": "success", "occurred_at": time.Now().UTC().Format(time.RFC3339)}
	batch := map[string]any{"events": []any{event}}
	result, _ = call("POST", "/agent/events/batch", batch, "", agent, false, 200)
	if result["data"].(map[string]any)["inserted"] != float64(1) {
		t.Fatal("event not inserted")
	}
	result, _ = call("POST", "/agent/events/batch", batch, "", agent, false, 200)
	if result["data"].(map[string]any)["duplicates"] != float64(1) {
		t.Fatal("dedup failed")
	}
	testAgentStream(t, handler, db, id, agent, event)
	testDeliveryLeases(t, db)
	testHealthRecovery(t, db, id, agent)
	channelBody := map[string]any{"name": "disabled local fixture", "kind": "webhook", "enabled": false, "config": map[string]string{"url": "https://127.0.0.1:1/fixture", "secret": "must-never-be-returned"}}
	channelResult, _ := call("POST", "/channels", channelBody, session, "", true, 200)
	channelID := channelResult["data"].(map[string]any)["id"].(string)
	channelResult, _ = call("GET", "/channels/"+channelID, nil, session, "", false, 200)
	encoded, _ := json.Marshal(channelResult)
	if bytes.Contains(encoded, []byte("must-never-be-returned")) || bytes.Contains(encoded, []byte("https://127.0.0.1")) {
		t.Fatal("channel secrets leaked")
	}
	masked := channelResult["data"].(map[string]any)
	if masked["configured"].(map[string]any)["secret"] != true {
		t.Fatal("missing masked-secret indicator")
	}
	channelBody["config"] = map[string]string{}
	call("PATCH", "/channels/"+channelID, channelBody, session, "", true, 200)
	call("POST", "/channels/"+channelID+"/test", map[string]any{}, session, "", true, 409)
	call("GET", "/rules", nil, session, "", false, 200)
	call("GET", "/notifications", nil, session, "", false, 200)
	event["outcome"] = "failure"
	call("POST", "/agent/events/batch", batch, "", agent, false, 400)
	call("GET", "/events?source_ip=2001:db8::1", nil, session, "", false, 200)
	call("GET", "/dashboard/summary", nil, session, "", false, 200)
	call("GET", "/dashboard/overview", nil, session, "", false, 200)
	for _, role := range []string{"viewer", "operator"} {
		name := role + uuid.NewString()[:8]
		call("POST", "/users", map[string]string{"username": name, "password": "viewer-test-password", "role": role}, session, "", true, 200)
		_, r := call("POST", "/auth/login", map[string]string{"username": name, "password": "viewer-test-password"}, "", "", true, 200)
		c := r.Result().Cookies()[0]
		s := c.Name + "=" + c.Value
		call("GET", "/servers", nil, s, "", false, 200)
		call("POST", "/servers/"+id+"/enrollment", map[string]string{}, s, "", true, 403)
		call("GET", "/audit-logs", nil, s, "", false, 403)
		call("GET", "/agent/update-commands", nil, s, "", false, 403)
		call("GET", "/channels", nil, s, "", false, 403)
		call("GET", "/rules", nil, s, "", false, 403)
		want := 403
		if role == "operator" {
			want = 200
		}
		call("POST", "/servers", map[string]string{"name": "role-check"}, s, "", true, want)
		call("POST", "/auth/password", map[string]string{"current_password": "wrong", "new_password": "new-password-2026"}, s, "", true, 403)
		call("POST", "/auth/password", map[string]string{"current_password": "viewer-test-password", "new_password": strings.Repeat("密", 30)}, s, "", true, 400)
		call("POST", "/auth/password", map[string]string{"current_password": "viewer-test-password", "new_password": "new-password-2026"}, s, "", true, 200)
		call("GET", "/auth/me", nil, s, "", false, 401)
		call("POST", "/auth/login", map[string]string{"username": name, "password": "viewer-test-password"}, "", "", true, 401)
		call("POST", "/auth/login", map[string]string{"username": name, "password": "new-password-2026"}, "", "", true, 200)
	}
	var adminID string
	if e = db.Row(ctx, `SELECT id FROM users WHERE username=?`, cfg.AdminUsername).Scan(&adminID); e != nil {
		t.Fatal(e)
	}
	expired := hash(uuid.NewString())
	if _, e = db.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)`, expired, adminID, time.Now().UTC().Add(-time.Hour)); e != nil {
		t.Fatal(e)
	}
	if e = storage.Cleanup(ctx, db); e != nil {
		t.Fatal(e)
	}
	var expiredCount int
	if e = db.Row(ctx, `SELECT COUNT(*) FROM sessions WHERE token_hash=?`, expired).Scan(&expiredCount); e != nil || expiredCount != 0 {
		t.Fatal("expired session retained", e)
	}
	call("GET", "/auth/me", nil, session, "", false, 200)
	call("POST", "/servers/"+id+"/revoke", map[string]string{}, session, "", true, 200)
	call("POST", "/agent/heartbeat", nil, "", agent, false, 401)
	call("POST", "/auth/logout", nil, session, "", true, 200)
	call("GET", "/auth/me", nil, session, "", false, 401)
	var count int
	if e = db.Row(ctx, `SELECT COUNT(*) FROM audit_logs WHERE target=?`, id).Scan(&count); e != nil || count < 4 {
		t.Fatal("audit missing", count, e)
	}
	name := "missing" + uuid.NewString()
	for i := 0; i < 11; i++ {
		want := 401
		if i == 10 {
			want = 429
		}
		call("POST", "/auth/login", map[string]string{"username": name, "password": "wrong-password"}, "", "", true, want)
	}
	fmt.Println("integration passed:", driver, "migration, auth, CSRF, RBAC, enrollment, replay, heartbeat, events, dedup, revocation, audit, rate limiting")
	if os.Getenv("SSHGUARD_LOAD_TEST") == "1" {
		testConcurrentAgents(t, db)
	}
}
