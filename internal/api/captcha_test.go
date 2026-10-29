package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

type captchaRoundTrip func(*http.Request) (*http.Response, error)

func (f captchaRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCaptchaProvidersValidateOnServer(t *testing.T) {
	now := time.Now().UTC()
	for _, provider := range []string{"turnstile", "recaptcha", "geetest"} {
		for _, scenario := range []string{"valid", "wrong-host", "expired", "bad-token", "outage", "oversize", "bad-json", "wrong-action", "wrong-lot"} {
			t.Run(provider+"/"+scenario, func(t *testing.T) {
				cfg := CaptchaConfig{Provider: provider, SiteKey: "site", Secret: "secret", Hostnames: []string{"guard.example.com"}}
				answer := CaptchaAnswer{Token: "proof", Lot: "lot", Pass: "pass", Output: "output", Time: strconv.FormatInt(now.Unix(), 10)}
				client := &http.Client{Transport: captchaRoundTrip(func(r *http.Request) (*http.Response, error) {
					if r.URL.Scheme != "https" || r.Method != "POST" {
						t.Fatal("unsafe verification request")
					}
					want := map[string]string{"turnstile": "challenges.cloudflare.com", "recaptcha": "www.google.com", "geetest": "gcaptcha4.geetest.com"}[provider]
					if r.URL.Host != want {
						t.Fatal("wrong verification endpoint", r.URL)
					}
					if e := r.ParseForm(); e != nil {
						t.Fatal(e)
					}
					if provider == "geetest" {
						mac := hmac.New(sha256.New, []byte("secret"))
						mac.Write([]byte("lot"))
						if r.Form.Get("sign_token") != hex.EncodeToString(mac.Sum(nil)) || r.Form.Get("captcha_id") != "site" {
							t.Fatal("GeeTest signature missing")
						}
					} else if r.Form.Get("secret") != "secret" || r.Form.Get("response") != "proof" || r.Form.Get("remoteip") != "192.0.2.1" {
						t.Fatal("verification fields missing")
					}
					host, at, action, result, lot := "guard.example.com", now, "login", "success", "lot"
					if scenario == "wrong-host" {
						host = "attacker.example"
					}
					if scenario == "expired" {
						at = now.Add(-6 * time.Minute)
					}
					if scenario == "wrong-action" {
						action = "other"
					}
					if scenario == "wrong-lot" {
						lot = "other"
					}
					if scenario == "bad-token" {
						result = "fail"
					}
					value := map[string]any{"success": scenario != "bad-token", "hostname": host, "challenge_ts": at, "action": action, "result": result, "captcha_args": map[string]any{"referer": "https://" + host + "/login", "lot_number": lot}}
					raw, _ := json.Marshal(value)
					status := 200
					if scenario == "outage" {
						status = 503
					}
					if scenario == "oversize" {
						raw = []byte(strings.Repeat("x", 20000))
					}
					if scenario == "bad-json" {
						raw = []byte("not json")
					}
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(raw))), Header: make(http.Header)}, nil
				})}
				err := verifyCaptcha(context.Background(), client, cfg, answer, "192.0.2.1", now)
				valid := scenario == "valid" || (scenario == "wrong-action" && provider != "turnstile") || (scenario == "wrong-lot" && provider != "geetest") || (scenario == "expired" && provider == "geetest")
				if valid && err != nil {
					t.Fatal("valid response refused", err)
				}
				if !valid && err == nil {
					t.Fatal("untrusted validation accepted")
				}
				if scenario == "outage" && !errors.Is(err, errCaptchaUnavailable) {
					t.Fatal("outage treated as login failure", err)
				}
			})
		}
	}
}
