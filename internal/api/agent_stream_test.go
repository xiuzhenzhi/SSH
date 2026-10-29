package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"sshguard/internal/storage"
)

func testAgentStream(t *testing.T, handler http.Handler, db *storage.DB, serverID, credential string, event any) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/agent/stream"
	conn, _, e := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + credential}}})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	exchange := func(kind string, data any, want int) agentAck {
		t.Helper()
		id := uuid.NewString()
		if e := wsjson.Write(ctx, conn, map[string]any{"request_id": id, "type": kind, "data": data}); e != nil {
			t.Fatal(e)
		}
		var ack agentAck
		if e := wsjson.Read(ctx, conn, &ack); e != nil {
			t.Fatal(e)
		}
		if ack.Status != want || ack.RequestID != id || ack.Type != "ack" {
			t.Fatalf("invalid ACK: %+v", ack)
		}
		return ack
	}
	ack := exchange("events", map[string]any{"events": []any{event}}, 200)
	b, _ := json.Marshal(ack.Data)
	var result BatchResult
	json.Unmarshal(b, &result)
	if result.Accepted != 1 || result.Inserted != 0 || result.Duplicates != 1 {
		t.Fatal("cross-transport duplicate not deduplicated", result)
	}
	var count int
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=?`, serverID).Scan(&count); e != nil || count != 1 {
		t.Fatal("duplicate generated repeated alerts", count, e)
	}
	exchange("events", map[string]any{"events": []any{map[string]any{"event_id": "bad"}}}, 400)
	ack = exchange("heartbeat", map[string]any{"version": "stream-test"}, 200)
	b, _ = json.Marshal(ack.Data)
	var settings map[string]any
	if e := json.Unmarshal(b, &settings); e != nil {
		t.Fatal(e)
	}
	if _, found := settings["report_interval_seconds"]; found {
		t.Fatal("WebSocket ACK must not reconfigure Agent")
	}
	// A browser origin must never bypass the native-only boundary.
	rejected, resp, e := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + credential}, "Origin": []string{"https://attacker.example"}}})
	if rejected != nil {
		rejected.CloseNow()
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if e == nil || resp == nil || resp.StatusCode != 403 {
		t.Fatal("browser Agent stream accepted")
	}
	// Revocation must apply to an already-open stream, not just the handshake.
	if _, e = db.Exec(ctx, `UPDATE agent_credentials SET revoked_at=? WHERE token_hash=?`, time.Now().UTC(), hash(credential)); e != nil {
		t.Fatal(e)
	}
	exchange("heartbeat", map[string]any{}, 401)
	if _, e = db.Exec(ctx, `UPDATE agent_credentials SET revoked_at=NULL WHERE token_hash=?`, hash(credential)); e != nil {
		t.Fatal(e)
	}
}
