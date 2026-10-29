// Package originurl validates the same platform address for the API and Agent.
package originurl

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

func Parse(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return nil, fmt.Errorf("expected an HTTP(S) origin without credentials, path, query or fragment")
	}
	host := u.Hostname()
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Zone() != "" || addr.IsUnspecified() || (addr.Is6() != strings.HasPrefix(u.Host, "[")) {
			return nil, fmt.Errorf("use a reachable IPv4 or bracketed IPv6 address")
		}
	} else {
		if strings.ContainsAny(host, ":%") || strings.HasPrefix(u.Host, "[") || len(host) > 253 || strings.Trim(host, "0123456789.") == "" {
			return nil, fmt.Errorf("invalid platform hostname")
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, fmt.Errorf("invalid platform hostname")
			}
			for _, ch := range label {
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-') {
					return nil, fmt.Errorf("use an ASCII hostname or its punycode form")
				}
			}
		}
	}
	if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("platform port must not be empty")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("platform port must be 1–65535")
		}
	}
	return u, nil
}

// Canonical matches browser Origin serialization, including default ports.
func Canonical(u *url.URL) string {
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if addr, err := netip.ParseAddr(host); err == nil {
		host = addr.String()
		if addr.Is4In6() {
			b := addr.As16()
			host = fmt.Sprintf("::ffff:%x:%x", uint16(b[12])<<8|uint16(b[13]), uint16(b[14])<<8|uint16(b[15]))
		}
	}
	if port != "" {
		n, _ := strconv.Atoi(port)
		port = strconv.Itoa(n)
	}
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return u.Scheme + "://" + host
}
