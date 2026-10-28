package notify

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecretsAreAuthenticatedAndBoundToChannel(t *testing.T) {
	key := strings.Repeat("ab", 32)
	sealed, e := Seal(key, "channel-a", []byte("password"))
	if e != nil {
		t.Fatal(e)
	}
	b, e := Open(key, "channel-a", sealed)
	if e != nil || string(b) != "password" {
		t.Fatal(e)
	}
	if _, e = Open(key, "channel-b", sealed); e == nil {
		t.Fatal("ciphertext could be swapped between channels")
	}
	if _, e = Open(strings.Repeat("cd", 32), "channel-a", sealed); e == nil {
		t.Fatal("wrong key accepted")
	}
}
func TestDestinationPolicy(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "::1", "10.1.1.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "::ffff:127.0.0.1", "fc00::1", "224.0.0.1"} {
		if publicIP(net.ParseIP(s)) {
			t.Fatal("unsafe address allowed", s)
		}
	}
	if !publicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	if _, e := (Sender{}).dial(context.Background(), "tcp", "127.0.0.1:1"); e == nil || !strings.Contains(e.Error(), "private") {
		t.Fatal("private address not rejected", e)
	}
}
func TestWebhookTLSAndAcknowledgement(t *testing.T) {
	status := 200
	response := `{"errcode":0}`
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		var value map[string]any
		if json.Unmarshal(b, &value) != nil {
			t.Error("invalid JSON")
		}
		if r.Header.Get("Idempotency-Key") != "test-id" {
			t.Error("missing stable delivery id")
		}
		w.WriteHeader(status)
		w.Write([]byte(response))
	}))
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	sender := Sender{AllowedHosts: []string{"127.0.0.1"}, RootCAs: pool}
	cfg := Config{URL: server.URL}
	m := Message{ID: "test-id", Title: "SSHGuard", Text: "root login"}
	if e := sender.Send(context.Background(), "wecom", cfg, m); e != nil {
		t.Fatal(e)
	}
	response = `{"errcode":40001}`
	if e := sender.Send(context.Background(), "wecom", cfg, m); e == nil {
		t.Fatal("provider error treated as success")
	}
	response = `{}`
	if e := sender.Send(context.Background(), "wecom", cfg, m); e == nil {
		t.Fatal("empty acknowledgement accepted")
	}
	status = 302
	if e := sender.Send(context.Background(), "webhook", cfg, m); e == nil {
		t.Fatal("redirect accepted")
	}
	if calls != 4 {
		t.Fatal("unexpected request count", calls)
	}
	untrusted := Sender{AllowedHosts: []string{"127.0.0.1"}}
	if e := untrusted.Send(context.Background(), "webhook", cfg, m); e == nil {
		t.Fatal("untrusted TLS accepted")
	}
}
func TestChannelValidation(t *testing.T) {
	for _, url := range []string{"http://example.com", "https://u:p@example.com/hook", "file:///etc/passwd"} {
		if Validate("webhook", Config{URL: url}) == nil {
			t.Fatal("invalid URL accepted")
		}
	}
	if Validate("smtp", Config{Host: "smtp.example.com", Port: 25, TLSMode: "none", From: "a@example.com", To: []string{"b@example.com"}}) == nil {
		t.Fatal("plaintext SMTP accepted")
	}
}
