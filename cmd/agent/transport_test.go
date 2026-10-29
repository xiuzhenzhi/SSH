package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestUploadCommitAckLossFallback(t *testing.T) {
	t.Run("HTTPS", func(t *testing.T) { testUploadCommitAckLossFallback(t, true) })
	t.Run("direct HTTP", func(t *testing.T) { testUploadCommitAckLossFallback(t, false) })
}
func testUploadCommitAckLossFallback(t *testing.T, useTLS bool) {
	var mu sync.Mutex
	seen := map[string]bool{}
	wsCount, httpCount := 0, 0
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing authorization")
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/api/v1/agent/stream" {
			conn, e := websocket.Accept(w, r, nil)
			if e != nil {
				return
			}
			defer conn.CloseNow()
			var msg struct {
				Data struct {
					Events []SSHEvent `json:"events"`
				} `json:"data"`
			}
			if e = wsjson.Read(r.Context(), conn, &msg); e != nil {
				return
			}
			mu.Lock()
			wsCount++
			for _, event := range msg.Data.Events {
				seen[event.ID] = true
			}
			mu.Unlock()
			return // Simulate committed database transaction followed by connection loss before ACK.
		}
		var body struct {
			Events []SSHEvent `json:"events"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
			return
		}
		mu.Lock()
		httpCount++
		duplicates := 0
		for _, event := range body.Events {
			if seen[event.ID] {
				duplicates++
			}
			seen[event.ID] = true
		}
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]int{"accepted": len(body.Events), "inserted": len(body.Events) - duplicates, "duplicates": duplicates}})
	}))
	if useTLS {
		server.StartTLS()
	} else {
		server.Start()
	}
	defer server.Close()
	old := client
	client = server.Client()
	defer func() { client = old }()
	queue, e := openSpool(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer queue.db.Close()
	event, ok := parseSSH("Accepted publickey for root from 192.0.2.1 port 12345 ssh2", "fixture", time.Now())
	if !ok {
		t.Fatal("parse")
	}
	if e = queue.append([]SSHEvent{event}, cursor{}); e != nil {
		t.Fatal(e)
	}
	tr := &transport{cfg: Config{Endpoint: server.URL, Token: "test-token", AllowInsecureHTTP: !useTLS}}
	defer tr.close()
	if _, e = uploadBatch(context.Background(), tr, queue); e != nil {
		t.Fatal(e)
	}
	if q, _ := queue.counts(); q != 0 {
		t.Fatal("acknowledged queue not drained")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 || wsCount != 1 || httpCount != 1 {
		t.Fatalf("events=%d ws=%d http=%d", len(seen), wsCount, httpCount)
	}
}

func TestUploadRetainsUnacknowledgedAndQuarantinesOnlyInvalid(t *testing.T) {
	for _, mode := range []string{"unauthorized", "unavailable", "invalid_ack", "mixed"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Events []SSHEvent `json:"events"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				switch mode {
				case "unauthorized":
					w.WriteHeader(401)
				case "unavailable":
					w.WriteHeader(503)
				case "invalid_ack":
					w.Write([]byte(`{"data":{"accepted":2,"inserted":0,"duplicates":0}}`))
				case "mixed":
					for _, event := range body.Events {
						if event.Username == "bad" {
							w.WriteHeader(400)
							return
						}
					}
					json.NewEncoder(w).Encode(map[string]any{"data": map[string]int{"accepted": len(body.Events), "inserted": len(body.Events), "duplicates": 0}})
				}
			}))
			defer server.Close()
			old := client
			client = server.Client()
			defer func() { client = old }()
			queue, e := openSpool(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			defer queue.db.Close()
			good, _ := parseSSH("Accepted publickey for root from 192.0.2.1 port 12345 ssh2", "good", time.Now())
			bad := good
			bad.ID = "invalid"
			bad.Username = "bad"
			if e = queue.append([]SSHEvent{good, bad}, cursor{}); e != nil {
				t.Fatal(e)
			}
			tr := &transport{cfg: Config{Endpoint: server.URL}, nextDial: time.Now().Add(time.Hour)}
			_, e = uploadBatch(context.Background(), tr, queue)
			q, d := queue.counts()
			if mode == "mixed" {
				if e != nil || q != 0 || d != 1 {
					t.Fatalf("q=%d rejected=%d err=%v", q, d, e)
				}
			} else if e == nil || q != 2 || d != 0 {
				t.Fatalf("unacknowledged events lost q=%d rejected=%d err=%v", q, d, e)
			}
		})
	}
}

func TestTransportRejectsMismatchedWebSocketAck(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/stream" {
			w.WriteHeader(503)
			return
		}
		conn, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.CloseNow()
		var msg map[string]any
		if wsjson.Read(r.Context(), conn, &msg) != nil {
			return
		}
		wsjson.Write(r.Context(), conn, map[string]any{"type": "ack", "request_id": "wrong", "status": 200, "data": map[string]any{"accepted": 1, "inserted": 1, "duplicates": 0}})
		<-conn.CloseRead(r.Context()).Done()
	}))
	defer server.Close()
	old := client
	client = server.Client()
	defer func() { client = old }()
	tr := &transport{cfg: Config{Endpoint: server.URL}}
	defer tr.close()
	var out any
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e := tr.send(ctx, "events", map[string]any{}, &out); e == nil {
		t.Fatal("accepted unrelated ACK")
	}
}
