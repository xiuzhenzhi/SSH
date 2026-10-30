package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sshguard/internal/ipaddress"
	"strings"
	"sync"
	"time"
)

type publicAddress struct {
	value string
	seen  time.Time
}
type addressResolver struct {
	mu         sync.Mutex
	next       time.Time
	refreshing bool
	public     [2]publicAddress
	probe      func(context.Context, int) (string, error)
	local      []string
	localNext  time.Time
	interfaces func() []string
	nextProbe  [2]time.Time
	failures   [2]int
}

func publicIPClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	return &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{
		Proxy: nil, DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
		TLSHandshakeTimeout: 3 * time.Second, DisableKeepAlives: true,
	}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

var publicIPClients = [2]*http.Client{publicIPClient("tcp4"), publicIPClient("tcp6")}

func probePublicIP(ctx context.Context, family int) (string, error) {
	endpoints := [2][]string{
		{"https://api.ipify.org", "https://ipv4.icanhazip.com"},
		{"https://api6.ipify.org", "https://ipv6.icanhazip.com"},
	}
	for _, endpoint := range endpoints[family] {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		resp, err := publicIPClients[family].Do(req)
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 128))
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			continue
		}
		value := strings.TrimSpace(string(raw))
		if !ipaddress.Public(value) {
			continue
		}
		ip := netip.MustParseAddr(value).Unmap()
		if ip.Is4() != (family == 0) {
			continue
		}
		return ip.String(), nil
	}
	return "", errors.New("public IP discovery unavailable")
}

// Discovery runs in the background and is cached. An unavailable IPv6 route or
// public-IP service must never delay log uploads or the installed heartbeat cadence.
func (r *addressResolver) addresses(ctx context.Context) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if !now.Before(r.localNext) {
		interfaces := r.interfaces
		if interfaces == nil {
			interfaces = hostAddresses
		}
		r.local = interfaces()
		r.localNext = now.Add(time.Minute)
	}
	values := make([]string, len(r.local), len(r.local)+2)
	copy(values, r.local)
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, address := range r.public {
		if address.value != "" && now.Sub(address.seen) < time.Hour && !seen[address.value] {
			values = append(values, address.value)
		}
	}
	if !r.refreshing && !now.Before(r.next) && ctx.Err() == nil {
		r.refreshing = true
		probe := r.probe
		if probe == nil {
			probe = probePublicIP
		}
		go r.refresh(ctx, probe)
	}
	// Reserve space for public discovery even on hosts with many bridges/NICs.
	if len(values) > 16 {
		values = append(values[:14], values[len(values)-2:]...)
	}
	return values
}

func (r *addressResolver) refresh(ctx context.Context, probe func(context.Context, int) (string, error)) {
	ctx, cancel := context.WithTimeout(ctx, 9*time.Second)
	defer cancel()
	var group sync.WaitGroup
	for family := 0; family < 2; family++ {
		r.mu.Lock()
		due := !time.Now().Before(r.nextProbe[family])
		r.mu.Unlock()
		if !due {
			continue
		}
		group.Add(1)
		go func(family int) {
			defer group.Done()
			value, err := probe(ctx, family)
			r.mu.Lock()
			defer r.mu.Unlock()
			if err == nil && ipaddress.Public(value) {
				r.public[family] = publicAddress{value: value, seen: time.Now()}
				r.failures[family] = 0
				r.nextProbe[family] = time.Now().Add(15 * time.Minute)
			} else {
				r.failures[family] = min(r.failures[family]+1, 5)
				r.nextProbe[family] = time.Now().Add(time.Duration(min(15, 1<<(r.failures[family]-1))) * time.Minute)
			}
		}(family)
	}
	group.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshing = false
	// Each family has its own cache/backoff, so missing IPv6 does not repeatedly
	// resolve a healthy IPv4 address or retain idle TLS connections.
	r.next = r.nextProbe[0]
	if r.nextProbe[1].Before(r.next) {
		r.next = r.nextProbe[1]
	}
}
