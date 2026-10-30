package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCollectorHealthDiagnostics(t *testing.T) {
	health := &collectorHealth{}
	health.update(errors.New(strings.Repeat("错误说明", 80)), time.Time{})
	message := health.heartbeat(nil)["collector_message"].(string)
	if len(message) > 256 || len(message) < 200 || !utf8.ValidString(message) {
		t.Fatal("diagnostic truncation broke Chinese text", len(message), message)
	}
	health.update(nil, time.Time{})
	for _, state := range []*spool{{queued: queueLimit}, {rejected: 1}} {
		payload := health.heartbeat(state)
		if payload["collector_state"] != "error" || payload["collector_message"] == "" {
			t.Fatal("queue error omitted its explanation", payload)
		}
	}
	health.update(errors.New("磁盘空间不足"), time.Time{})
	if got := health.heartbeat(&spool{queued: queueLimit})["collector_message"]; got != "磁盘空间不足" {
		t.Fatal("queue warning replaced the underlying storage error", got)
	}
}

func TestAgentStateInitializationHealth(t *testing.T) {
	for _, stateFailure := range []bool{false, true} {
		name := "recovers mismatched state"
		if stateFailure {
			name = "reports underlying state error"
		}
		t.Run(name, func(t *testing.T) {
			seen := make(chan map[string]any, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/agent/stream" {
					w.WriteHeader(503)
					return
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if r.URL.Path != "/api/v1/agent/heartbeat" {
					t.Errorf("old queued events reached the new server: %s", r.URL.Path)
				}
				json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"received": true}})
				select {
				case seen <- payload:
				default:
				}
			}))
			defer server.Close()
			oldClient := client
			client = server.Client()
			defer func() { client.CloseIdleConnections(); client = oldClient }()
			dir := t.TempDir()
			logPath := filepath.Join(dir, "auth.log")
			if err := os.WriteFile(logPath, nil, 0600); err != nil {
				t.Fatal(err)
			}
			stateDir := filepath.Join(dir, "state")
			if stateFailure {
				if err := os.WriteFile(stateDir, []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				s, err := openSpool(stateDir)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.bindIdentity(Config{Endpoint: server.URL, ServerID: "old-server"}); err == nil {
					err = s.append([]SSHEvent{{ID: "old-event"}}, cursor{Kind: "file", Offset: 100})
				}
				s.db.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				runAgent(ctx, Config{Endpoint: server.URL, ServerID: "new-server", Token: strings.Repeat("b", 64), AllowInsecureHTTP: true, LogFile: logPath}, stateDir)
			}()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("Agent did not stop")
				}
			}()
			select {
			case payload := <-seen:
				if stateFailure {
					message, _ := payload["collector_message"].(string)
					if payload["collector_state"] != "error" || !strings.Contains(message, "cannot initialize durable Agent state") || !strings.Contains(message, "create state directory") {
						t.Fatal("heartbeat omitted the specific database error", payload)
					}
				} else if payload["collector_state"] != "healthy" || payload["queue_depth"] != float64(0) || payload["rejected_events"] != float64(0) {
					t.Fatal("new identity did not recover collection", payload)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Agent did not report initialization health")
			}
		})
	}
}
