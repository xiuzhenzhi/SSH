package api

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Driver, DatabaseURL, PublicURL, Listen, AdminUsername, AdminPassword string
	SecureCookie                                                         bool
}

func LoadConfig() (Config, error) {
	c := Config{Driver: os.Getenv("DB_DRIVER"), DatabaseURL: os.Getenv("DATABASE_URL"), PublicURL: strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"), Listen: os.Getenv("LISTEN_ADDR"), AdminUsername: os.Getenv("ADMIN_USERNAME"), AdminPassword: os.Getenv("ADMIN_PASSWORD"), SecureCookie: true}
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
