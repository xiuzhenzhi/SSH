package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestDiagnosticOutputStaysBounded(t *testing.T) {
	var buffer diagnosticBuffer
	chunk := bytes.Repeat([]byte("x"), 64<<10)
	for i := 0; i < 320; i++ {
		if n, err := buffer.Write(chunk); err != nil || n != len(chunk) {
			t.Fatal("diagnostic stream was not drained")
		}
	}
	if len(buffer.data) != diagnosticLimit || cap(buffer.data) > diagnosticLimit*2 {
		t.Fatal("diagnostic output grew without a bound")
	}
}

func TestReconnectReleasesResources(t *testing.T) {
	var active, attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/stream" {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			active.Add(1)
			attempts.Add(1)
			defer active.Add(-1)
			defer conn.CloseNow()
			var message any
			_ = wsjson.Read(r.Context(), conn, &message)
			return // Drop every connection before its acknowledgement.
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"data":{"received":true}}`)
	}))
	defer server.Close()
	previous := client
	client = server.Client()
	defer func() { client = previous }()
	tr := &transport{cfg: Config{Endpoint: server.URL, AllowInsecureHTTP: true}}
	defer tr.close()
	round := func() {
		t.Helper()
		tr.nextDial = time.Time{}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := tr.send(ctx, "heartbeat", map[string]any{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 20; i++ {
		round()
	}
	client.CloseIdleConnections()
	server.CloseClientConnections()
	runtime.GC()
	time.Sleep(30 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 300; i++ {
		round()
	}
	tr.close()
	client.CloseIdleConnections()
	server.CloseClientConnections()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && (active.Load() != 0 || runtime.NumGoroutine() > baseline+8) {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if active.Load() != 0 || runtime.NumGoroutine() > baseline+8 {
		t.Fatalf("connections/goroutines retained: active=%d baseline=%d now=%d", active.Load(), baseline, runtime.NumGoroutine())
	}
	if after.HeapAlloc > before.HeapAlloc+(4<<20) {
		t.Fatalf("heap retained after reconnects: before=%d after=%d", before.HeapAlloc, after.HeapAlloc)
	}
	if attempts.Load() != 320 {
		t.Fatalf("reconnect path not exercised: %d", attempts.Load())
	}
	t.Logf("300 reconnects: goroutines %d -> %d; live heap %d -> %d bytes", baseline, runtime.NumGoroutine(), before.HeapAlloc, after.HeapAlloc)
}

func TestAgentCancellationClosesStateAndCollector(t *testing.T) {
	seen := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/stream" {
			w.WriteHeader(503)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, `{"data":{"received":true}}`)
		select {
		case seen <- struct{}{}:
		default:
		}
	}))
	defer server.Close()
	previous := client
	client = server.Client()
	defer func() { client.CloseIdleConnections(); client = previous }()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.log")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			runAgent(ctx, Config{Endpoint: server.URL, ServerID: "fixture", LogFile: path, AllowInsecureHTTP: true}, state)
		}()
		select {
		case <-seen:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("agent did not start")
		}
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("agent collector did not stop")
		}
		// A leaked bbolt handle keeps its exclusive lock and prevents reopening.
		s, err := openSpool(state)
		if err != nil {
			t.Fatal("state handle not released", err)
		}
		s.db.Close()
	}
}
