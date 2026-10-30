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
}

func publicIPClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	return &http.Client{Timeout: 4 * time.Second, Transport: &http.Transport{
		Proxy: nil, DialContext: func(ctx context.Context, _ string, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
		TLSHandshakeTimeout: 3 * time.Second, IdleConnTimeout: time.Minute,
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
	values := hostAddresses()
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
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
		group.Add(1)
		go func(family int) {
			defer group.Done()
			value, err := probe(ctx, family)
			if err == nil && ipaddress.Public(value) {
				r.mu.Lock()
				r.public[family] = publicAddress{value: value, seen: time.Now()}
				r.mu.Unlock()
			}
		}(family)
	}
	group.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refreshing = false
	// Retry missing families; stable dual-stack results need only one refresh per 15 minutes.
	delay := 15 * time.Minute
	for _, address := range r.public {
		if address.value == "" || time.Since(address.seen) > time.Minute {
			delay = time.Minute
		}
	}
	r.next = time.Now().Add(delay)
}
