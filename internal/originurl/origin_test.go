package originurl

import "testing"

func TestParseAndBrowserCanonicalization(t *testing.T) {
	for raw, want := range map[string]string{
		"http://[2001:0db8:0:0:0:0:0:1]:80": "http://[2001:db8::1]",
		"https://[::ffff:192.0.2.128]":      "https://[::ffff:c000:280]",
		"https://Guard.Example.com:443":     "https://guard.example.com",
		"http://localhost:080":              "http://localhost",
		"https://192.0.2.10:8443":           "https://192.0.2.10:8443",
		"http://[2001:db8::1]:8080":         "http://[2001:db8::1]:8080",
	} {
		u, err := Parse(raw)
		if err != nil || Canonical(u) != want {
			t.Errorf("%s: %v", raw, err)
		}
	}
	for _, raw := range []string{
		"https://guard.example.com:", "http://192.0.2.10:0", "http://192.0.2.10:65536",
		"https://guard.example.com?", "https://guard.example.com#", "https://guard.example.com/path",
		"https://user:pass@guard.example.com", "http://[127.0.0.1]", "http://2001:db8::1",
		"http://127.1", "http://0.0.0.0:8080", "http://[::]:8080", "http://[fe80::1%25eth0]",
		"https://bad..example", "https://-bad.example", "https://bad_.example", "https://example.com\\evil",
	} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
