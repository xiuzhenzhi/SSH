package api

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuiltInWeb(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"assets", "downloads"} {
		if err := os.Mkdir(filepath.Join(dir, folder), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"index.html": "<html>console</html>", "assets/app.js": "console.log('ready')", "downloads/install-agent.sh": "#!/bin/sh\n", ".env": "secret", "database.db": "secret"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{cfg: Config{WebDir: dir}}
	router := gin.New()
	router.NoRoute(app.serveWeb)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/", 200}, {"HEAD", "/", 200}, {"GET", "/servers", 200},
		{"GET", "/assets/app.js", 200}, {"GET", "/downloads/install-agent.sh", 200},
		{"GET", "/assets/missing.js", 404}, {"GET", "/downloads/", 404},
		{"GET", "/api/missing", 404}, {"GET", "/.env", 404}, {"GET", "/database.db", 404},
		{"GET", "/assets/%2e%2e/.env", 404}, {"POST", "/", 404},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.status {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "secret") {
				t.Fatal("private file leaked")
			}
			if tc.method == "HEAD" && rec.Body.Len() != 0 {
				t.Fatal("HEAD returned body")
			}
		})
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "assets", "escape.txt")); err == nil {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("GET", "/assets/escape.txt", nil))
		if rec.Code != 404 {
			t.Fatal("symlink escaped web root")
		}
	}
}

func TestStandaloneHTTPRequiresExplicitOptIn(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("DATABASE_URL", "file:test.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("PUBLIC_URL", "http://192.168.1.10:8080")
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	t.Setenv("ALLOW_INSECURE_HTTP", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("public HTTP accepted without opt-in")
	}
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	if cfg, err := LoadConfig(); err != nil || cfg.SecureCookie {
		t.Fatalf("HTTP opt-in: %v", err)
	}
	t.Setenv("TLS_CERT_FILE", "cert.pem")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("partial TLS configuration accepted")
	}
	t.Setenv("TLS_KEY_FILE", "key.pem")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("TLS with HTTP origin accepted")
	}
	t.Setenv("PUBLIC_URL", "https://guard.example.com:8080")
	if cfg, err := LoadConfig(); err != nil || !cfg.SecureCookie {
		t.Fatalf("HTTPS config: %v", err)
	}
}
