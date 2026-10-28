package api

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Driver, DatabaseURL, PublicURL, Listen, AdminUsername, AdminPassword string
	SecureCookie                                                         bool
	EncryptionKey                                                        string
	NotificationAllowedHosts                                             []string
	EventRetentionDays, AlertRetentionDays, AuditRetentionDays           int
}

func LoadConfig() (Config, error) {
	c := Config{Driver: os.Getenv("DB_DRIVER"), DatabaseURL: os.Getenv("DATABASE_URL"), PublicURL: strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"), Listen: os.Getenv("LISTEN_ADDR"), AdminUsername: os.Getenv("ADMIN_USERNAME"), AdminPassword: os.Getenv("ADMIN_PASSWORD"), SecureCookie: true}
	c.EncryptionKey = os.Getenv("ENCRYPTION_KEY")
	for _, setting := range []struct {
		name     string
		target   *int
		fallback int
	}{{"EVENT_RETENTION_DAYS", &c.EventRetentionDays, 30}, {"ALERT_RETENTION_DAYS", &c.AlertRetentionDays, 90}, {"AUDIT_RETENTION_DAYS", &c.AuditRetentionDays, 180}} {
		*setting.target = setting.fallback
		if raw := os.Getenv(setting.name); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 30 || n > 3650 {
				return c, fmt.Errorf("%s must be between 30 and 3650", setting.name)
			}
			*setting.target = n
		}
	}
	if key, e := hex.DecodeString(c.EncryptionKey); e != nil || len(key) != 32 {
		return c, fmt.Errorf("ENCRYPTION_KEY must be 64 hex characters; preserve it with database backups")
	}
	if hosts := os.Getenv("NOTIFICATION_ALLOWED_HOSTS"); hosts != "" {
		c.NotificationAllowedHosts = strings.Split(hosts, ",")
	}
	if c.Driver == "" {
		c.Driver = "postgres"
	}
	if c.Driver != "postgres" && c.Driver != "mysql" {
		return c, fmt.Errorf("DB_DRIVER must be postgres or mysql")
	}
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.AdminUsername == "" {
		c.AdminUsername = "admin"
	}
	u, e := url.Parse(c.PublicURL)
	if e != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return c, fmt.Errorf("PUBLIC_URL must be an origin, e.g. https://guard.example.com")
	}
	if u.Scheme != "https" {
		if u.Scheme != "http" || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
			return c, fmt.Errorf("public deployments require HTTPS")
		}
		c.SecureCookie = false
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL required")
	}
	return c, nil
}
