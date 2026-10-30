package ipaddress

import "net/netip"

var special = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:10::/28"), netip.MustParsePrefix("2001:20::/28"),
	netip.MustParsePrefix("3fff::/20"),
}

var globalIPv6 = netip.MustParsePrefix("2000::/3")

// Public excludes LAN, container, carrier NAT and reserved addresses. Go's
// IsGlobalUnicast alone deliberately includes private addresses.
func Public(value string) bool {
	ip, err := netip.ParseAddr(value)
	if err != nil || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	if ip.Is6() && !globalIPv6.Contains(ip) {
		return false
	}
	for _, prefix := range special {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}
