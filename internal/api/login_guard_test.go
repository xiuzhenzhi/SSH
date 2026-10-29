package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"sshguard/internal/storage"
)

func testLoginProtection(t *testing.T, db *storage.DB, cfg Config, session string) {
	t.Helper()
	ctx := context.Background()
	app := NewApp(db, cfg)
	handler := app.Router()
	call := func(ip, method, path string, body any, cookie string, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		request := httptest.NewRequest(method, "http://localhost/api/v1"+path, bytes.NewReader(b))
		request.RemoteAddr = ip + ":1234"
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", cfg.PublicURL)
		request.Header.Set("Cookie", cookie)
		request.Header.Set("X-Forwarded-For", "203.0.113.66")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("%s got %d want %d: %s", path, response.Code, want, response.Body.String())
		}
		var value map[string]any
		json.Unmarshal(response.Body.Bytes(), &value)
		return value
	}
	password := map[string]any{"username": "admin", "password": cfg.AdminPassword}
	for i := 0; i < 5; i++ {
		want := 401
		if i == 4 {
			want = 429
		}
		call("198.51.100.101", "POST", "/auth/login", map[string]any{"username": "unknown-guard", "password": "wrong-password"}, "", want)
	}
	call("198.51.100.101", "POST", "/auth/login", password, "", 429)
	until, e := NewApp(db, cfg).loginBlocked(ctx, "198.51.100.101")
	if e != nil || time.Until(until) < 23*time.Hour {
		t.Fatal("24 hour block did not survive application restart", until, e)
	}
	call("198.51.100.102", "POST", "/auth/login", password, "", 200)
	call("198.51.100.102", "DELETE", "/auth/blocked-ips/"+hash("198.51.100.101"), nil, session, 200)
	call("198.51.100.101", "POST", "/auth/login", password, "", 200)
	// The rolling window excludes failures that are older than 60 seconds.
	for i := 0; i < 4; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Request.RemoteAddr = "198.51.100.103:1234"
		if _, e := app.recordLoginFailure(c, "198.51.100.103"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := db.Exec(ctx, `UPDATE login_failures SET occurred_at=? WHERE source_hash=?`, time.Now().UTC().Add(-61*time.Second), hash("198.51.100.103")); e != nil {
		t.Fatal(e)
	}
	call("198.51.100.103", "POST", "/auth/login", map[string]any{"username": "rolling-guard", "password": "wrong-password"}, "", 401)
	// Concurrent failures must create one block and keep only five counted failures.
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/", nil)
			c.Request.RemoteAddr = "198.51.100.104:1234"
			if _, e := app.recordLoginFailure(c, "198.51.100.104"); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	var count int
	if e := db.Row(ctx, `SELECT COUNT(*) FROM login_failures WHERE source_hash=?`, hash("198.51.100.104")).Scan(&count); e != nil || count != 5 {
		t.Fatal("failure race", count, e)
	}
	if e := db.Row(ctx, `SELECT COUNT(*) FROM audit_logs WHERE action='auth.ip.block' AND target=?`, "198.51.100.104").Scan(&count); e != nil || count != 1 {
		t.Fatal("block audit repeated", count, e)
	}
	if _, e := db.Exec(ctx, `UPDATE login_guards SET blocked_until=? WHERE source_hash=?`, time.Now().UTC().Add(-time.Second), hash("198.51.100.104")); e != nil {
		t.Fatal(e)
	}
	call("198.51.100.104", "POST", "/auth/login", password, "", 200)
	// Secret values never return to either public or authenticated settings reads.
	settings := map[string]any{"provider": "turnstile", "site_key": "site", "secret": "test-secret-never-return", "hostnames": []string{"guard.example.com"}, "current_password": cfg.AdminPassword}
	call("198.51.100.102", "PATCH", "/auth/protection/settings", settings, session, 200)
	for _, path := range []string{"/auth/protection", "/auth/protection/settings"} {
		value := call("198.51.100.102", "GET", path, nil, session, 200)
		raw, _ := json.Marshal(value)
		if strings.Contains(string(raw), "test-secret-never-return") {
			t.Fatal("captcha secret exposed")
		}
	}
	var ciphertext string
	if e := db.Row(ctx, `SELECT config_ciphertext FROM login_protection WHERE id=1`).Scan(&ciphertext); e != nil || strings.Contains(ciphertext, "test-secret") {
		t.Fatal("captcha secret not encrypted", e)
	}
	call("198.51.100.105", "POST", "/auth/login", password, "", 400)
	previous := captchaHTTP
	captchaHTTP = &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
		raw, _ := json.Marshal(map[string]any{"success": true, "hostname": "guard.example.com", "action": "login", "challenge_ts": time.Now().UTC()})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(raw)), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { captchaHTTP = previous })
	withProof := map[string]any{"username": "admin", "password": "wrong-password", "captcha": map[string]any{"token": "proof-consumed-by-test"}}
	call("198.51.100.106", "POST", "/auth/login", withProof, "", 401)
	withProof["password"] = cfg.AdminPassword
	call("198.51.100.106", "POST", "/auth/login", withProof, "", 400)
	withProof["captcha"] = map[string]any{"token": "fresh-proof"}
	call("198.51.100.106", "POST", "/auth/login", withProof, "", 200)
	captchaHTTP = &http.Client{Transport: captchaRoundTrip(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("outage")), Header: make(http.Header)}, nil
	})}
	withProof["captcha"] = map[string]any{"token": "outage-proof"}
	call("198.51.100.107", "POST", "/auth/login", withProof, "", 503)
	if e := db.Row(ctx, `SELECT COUNT(*) FROM login_failures WHERE source_hash=?`, hash("198.51.100.107")).Scan(&count); e != nil || count != 0 {
		t.Fatal("provider outage penalized client", count, e)
	}
	captchaHTTP = previous
	call("198.51.100.102", "PATCH", "/auth/protection/settings", map[string]any{"provider": "none", "current_password": cfg.AdminPassword}, session, 200)
}
