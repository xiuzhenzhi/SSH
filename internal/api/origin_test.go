package api

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDirectOriginWithoutDomain(t *testing.T) {
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("DATABASE_URL", "file:test.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("TLS_CERT_FILE", "")
	t.Setenv("TLS_KEY_FILE", "")
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	cfg, err := LoadConfig()
	if err != nil || cfg.SecureCookie {
		t.Fatalf("direct configuration: %v", err)
	}
	a := &App{cfg: cfg}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "http://192.0.2.10:8080/api/v1/auth/login", nil)
	c.Request.Header.Set("X-Forwarded-Host", "attacker.example")
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	if a.browserOrigin(c) != "http://192.0.2.10:8080" {
		t.Fatal("untrusted proxy headers changed origin")
	}
	for _, origin := range []string{"", "null", "https://attacker.example", "http://192.0.2.10:8081"} {
		if origin == a.browserOrigin(c) {
			t.Fatal("cross origin accepted")
		}
	}
	a.cfg.PublicURL = "https://guard.example.com"
	if a.browserOrigin(c) != a.cfg.PublicURL {
		t.Fatal("configured reverse proxy origin ignored")
	}
	t.Setenv("ALLOW_INSECURE_HTTP", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("implicit insecure mode allowed")
	}
}
