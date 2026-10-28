package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"sshguard/internal/storage"
)

func testConcurrentAgents(t *testing.T, db *storage.DB) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := NewApp(db, Config{PublicURL: "https://guard.example.com"})
	server := httptest.NewServer(app.Router())
	defer server.Close()
	defer app.Shutdown()
	group := "load-" + uuid.NewString()
	credentials := make([]string, 70)
	for i := range credentials {
		id, credential := uuid.NewString(), token()
		credentials[i] = credential
		if _, e := db.Exec(ctx, `INSERT INTO servers(id,name,group_name) VALUES(?,?,?)`, id, fmt.Sprintf("isolated load fixture %d", i), group); e != nil {
			t.Fatal(e)
		}
		if _, e := db.Exec(ctx, `INSERT INTO agent_credentials(token_hash,server_id) VALUES(?,?)`, hash(credential), id); e != nil {
			t.Fatal(e)
		}
	}
	started := time.Now()
	var wg sync.WaitGroup
	failures := make(chan error, len(credentials))
	for i, credential := range credentials {
		wg.Add(1)
		go func(i int, credential string) {
			defer wg.Done()
			run := func() error {
				conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/agent/stream", &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + credential}}})
				if e != nil {
					return e
				}
				defer conn.CloseNow()
				events := make([]Event, 10)
				for j := range events {
					events[j] = Event{ID: uuid.NewString(), Type: "ssh.login", Username: "load-fixture", IP: fmt.Sprintf("192.0.2.%d", i+1), Outcome: "success", At: time.Now().UTC()}
				}
				body := EventBatch{Events: events}
				id := uuid.NewString()
				if e = wsjson.Write(ctx, conn, map[string]any{"type": "events", "request_id": id, "data": body}); e != nil {
					return e
				}
				var ack agentAck
				if e = wsjson.Read(ctx, conn, &ack); e != nil {
					return e
				}
				if ack.Status != 200 || ack.RequestID != id {
					return fmt.Errorf("stream status %d", ack.Status)
				}
				b, _ := json.Marshal(body)
				req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/v1/agent/events/batch", bytes.NewReader(b))
				req.Header.Set("Authorization", "Bearer "+credential)
				req.Header.Set("Content-Type", "application/json")
				response, e := server.Client().Do(req)
				if e != nil {
					return e
				}
				defer response.Body.Close()
				var envelope struct {
					Data BatchResult `json:"data"`
				}
				if e = json.NewDecoder(response.Body).Decode(&envelope); e != nil {
					return e
				}
				if response.StatusCode != 200 || envelope.Data.Duplicates != 10 || envelope.Data.Inserted != 0 {
					return fmt.Errorf("HTTP replay status %d result %+v", response.StatusCode, envelope.Data)
				}
				return nil
			}
			if e := run(); e != nil {
				failures <- e
			}
		}(i, credential)
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		t.Error(e)
	}
	var count, alerts int
	if e := db.Row(ctx, `SELECT COUNT(*) FROM ssh_events e JOIN servers s ON s.id=e.server_id WHERE s.group_name=?`, group).Scan(&count); e != nil {
		t.Fatal(e)
	}
	if e := db.Row(ctx, `SELECT COUNT(*) FROM alerts a JOIN servers s ON s.id=a.server_id WHERE s.group_name=?`, group).Scan(&alerts); e != nil {
		t.Fatal(e)
	}
	if count != 700 || alerts != 70 {
		t.Fatalf("concurrent ingestion events=%d alerts=%d", count, alerts)
	}
	t.Logf("70 concurrent Agent streams: 700 events + 700 HTTPS replays; %s, 700 unique events / 70 cooldown alerts", time.Since(started).Round(time.Millisecond))
}
