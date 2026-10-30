package ipaddress

import "testing"

func TestPublicAddresses(t *testing.T) {
	for _, value := range []string{"8.8.8.8", "9.9.9.9", "2606:4700:4700::1111", "::ffff:8.8.8.8"} {
		if !Public(value) {
			t.Errorf("public IP rejected: %s", value)
		}
	}
	for _, value := range []string{"172.17.0.1", "172.18.0.1", "172.19.0.1", "10.0.0.2", "192.168.1.1", "100.64.0.1", "127.0.0.1", "0.0.0.0", "169.254.1.1", "224.0.0.1", "255.255.255.255", "198.18.0.1", "192.0.2.1", "::1", "fc00::1", "fe80::1", "ff02::1", "2001:db8::1", "3fff::1", "::ffff:10.0.0.2", "bad-ip"} {
		if Public(value) {
			t.Errorf("nonpublic IP accepted: %s", value)
		}
	}
}
