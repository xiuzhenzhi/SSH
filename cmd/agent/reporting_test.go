package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAgentKeepsInstalledInterval(t *testing.T) {
	seen := make(chan time.Time, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/stream" {
			w.WriteHeader(503)
			return
		}
		var payload map[string]any
		if e := json.NewDecoder(r.Body).Decode(&payload); e != nil {
			t.Error(e)
		}
		if payload["report_interval_seconds"] != float64(1) {
			t.Error("installed interval changed", payload)
		}
		select {
		case seen <- time.Now():
		default:
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"received": true, "report_interval_seconds": 30, "command": "must be ignored"}})
	}))
	defer server.Close()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.log")
	if e := os.WriteFile(path, nil, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		runAgent(ctx, Config{Endpoint: server.URL, ServerID: "fixed-interval-fixture", AllowInsecureHTTP: true, LogFile: path, ReportIntervalSeconds: 1}, filepath.Join(dir, "state"))
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-seen:
		case <-time.After(3 * time.Second):
			cancel()
			t.Fatal("remote ACK changed the installed cadence")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Agent did not release reporting loop")
	}
}

func TestFilePollWithoutChangesDoesNotWriteState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.log")
	if e := os.WriteFile(path, nil, 0600); e != nil {
		t.Fatal(e)
	}
	s, e := openSpool(filepath.Join(dir, "state"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.db.Close()
	if _, e := collectFile(context.Background(), s, path); e != nil {
		t.Fatal(e)
	}
	before := s.db.Stats().TxStats.Write
	for i := 0; i < 100; i++ {
		if _, e := collectFile(context.Background(), s, path); e != nil {
			t.Fatal(e)
		}
	}
	if after := s.db.Stats().TxStats.Write; after != before {
		t.Fatal("idle polling wrote durable state", before, after)
	}
}
