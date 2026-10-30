package geoip

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOnlineProviders(t *testing.T) {
	for _, tc := range []struct{ provider, token, host, body string }{
		{"geojs", "", "get.geojs.io", `{"country_code":"NL","country":"Netherlands"}`},
		{"ip-api", "", "ip-api.com", `{"status":"success","countryCode":"NL","country":"Netherlands"}`},
		{"ip-api", "secret-key", "pro.ip-api.com", `{"status":"success","countryCode":"NL","country":"Netherlands"}`},
		{"ipinfo", "", "ipinfo.io", `{"country":"NL"}`},
		{"ipinfo", "secret-key", "api.ipinfo.io", `{"country_code":"NL","country":"Netherlands"}`},
	} {
		t.Run(tc.provider+tc.token, func(t *testing.T) {
			client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != tc.host || !strings.Contains(r.URL.Path, "8.8.8.8") {
					t.Fatal("wrong endpoint", r.URL)
				}
				if tc.token != "" && r.URL.Scheme != "https" {
					t.Fatal("credentials sent without TLS")
				}
				if tc.provider == "ipinfo" && tc.token != "" && r.Header.Get("Authorization") != "Bearer secret-key" {
					t.Fatal("missing token")
				}
				if tc.provider == "ip-api" && tc.token != "" && r.URL.Query().Get("key") != "secret-key" {
					t.Fatal("missing pro key")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}
			value, err := Lookup(context.Background(), client, Config{Provider: tc.provider, Token: tc.token}, "8.8.8.8")
			if err != nil || value.Code != "NL" {
				t.Fatal(value, err)
			}
		})
	}
}

func TestProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		body   string
		status int
	}{
		{`{"country_code":"ZZ"}`, 200}, {`{"country_code":"not-a-country"}`, 200}, {`{"error":"bad token"}`, 200}, {`not json`, 200}, {strings.Repeat("x", 65537), 200}, {`denied`, 429},
	} {
		client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		if _, err := Lookup(context.Background(), client, Config{Provider: "geojs"}, "8.8.8.8"); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
	client := &http.Client{Transport: roundTrip(func(*http.Request) (*http.Response, error) { t.Fatal("private IP sent to service"); return nil, nil })}
	if _, err := Lookup(context.Background(), client, Config{Provider: "geojs"}, "10.0.0.2"); err == nil {
		t.Fatal("private IP accepted")
	}
	if _, err := ValidateDatabase([]byte("not mmdb")); err == nil {
		t.Fatal("bad database accepted")
	}
}
