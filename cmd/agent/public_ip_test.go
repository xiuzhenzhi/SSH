package main

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicDiscoveryDoesNotBlockHeartbeat(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gate := make(chan struct{})
	r := addressResolver{probe: func(ctx context.Context, family int) (string, error) {
		select {
		case <-gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if family == 0 {
			return "8.8.8.8", nil
		}
		return "2606:4700:4700::1111", nil
	}}
	start := time.Now()
	_ = r.addresses(ctx)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("public lookup blocks reporting")
	}
	close(gate)
	deadline := time.Now().Add(time.Second)
	for {
		r.mu.Lock()
		done := !r.refreshing
		r.mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lookup not completed")
		}
		time.Sleep(time.Millisecond)
	}
	values := r.addresses(ctx)
	if !slices.Contains(values, "8.8.8.8") || !slices.Contains(values, "2606:4700:4700::1111") || len(values) > 16 {
		t.Fatal("public addresses missing", values)
	}
	r.mu.Lock()
	next := r.next
	r.mu.Unlock()
	if time.Until(next) < 14*time.Minute {
		t.Fatal("discovery not cached")
	}
}

func TestLocalAddressesAreCachedAndIsolated(t *testing.T) {
	calls := 0
	r := addressResolver{next: time.Now().Add(time.Hour), interfaces: func() []string {
		calls++
		return []string{"8.8.8.8"}
	}}
	values := r.addresses(context.Background())
	values[0] = "mutated"
	for i := 0; i < 20; i++ {
		if values := r.addresses(context.Background()); len(values) != 1 || values[0] != "8.8.8.8" {
			t.Fatal("caller mutated the address cache", values)
		}
	}
	if calls != 1 {
		t.Fatal("interfaces enumerated on every heartbeat", calls)
	}
	r.localNext = time.Time{}
	r.addresses(context.Background())
	if calls != 2 {
		t.Fatal("interface cache did not refresh")
	}
}

func TestPublicDiscoverySchedulesFamiliesIndependently(t *testing.T) {
	var calls [2]atomic.Int64
	r := addressResolver{}
	probe := func(_ context.Context, family int) (string, error) {
		calls[family].Add(1)
		if family == 0 {
			return "8.8.8.8", nil
		}
		return "", errors.New("no IPv6 route")
	}
	r.refresh(context.Background(), probe)
	if calls[0].Load() != 1 || calls[1].Load() != 1 {
		t.Fatal("initial discovery did not probe both families")
	}
	for i := 0; i < 5; i++ {
		r.nextProbe[1] = time.Time{}
		r.refresh(context.Background(), probe)
	}
	if calls[0].Load() != 1 || calls[1].Load() != 6 {
		t.Fatal("missing IPv6 repeatedly probed healthy IPv4")
	}
	if time.Until(r.nextProbe[1]) < 14*time.Minute {
		t.Fatal("unavailable address family did not back off")
	}
}

func TestPublicDiscoveryFailurePreservesRecentSuccess(t *testing.T) {
	r := addressResolver{public: [2]publicAddress{{value: "8.8.8.8", seen: time.Now()}, {value: "2606:4700:4700::1111", seen: time.Now().Add(-2 * time.Hour)}}}
	r.refresh(context.Background(), func(context.Context, int) (string, error) { return "", errors.New("offline") })
	values := r.addresses(context.Background())
	if !slices.Contains(values, "8.8.8.8") || slices.Contains(values, "2606:4700:4700::1111") {
		t.Fatal("bad cache expiry", values)
	}
	if time.Until(r.next) > 2*time.Minute {
		t.Fatal("failure retries too slowly")
	}
}
