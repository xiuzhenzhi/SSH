package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type agentMessage struct {
	RequestID string          `json:"request_id"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
}
type agentAck struct {
	RequestID string `json:"request_id"`
	Type      string `json:"type"`
	Status    int    `json:"status"`
	Data      any    `json:"data,omitempty"`
}

func (h *liveHub) addAgent(id string, conn *websocket.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.agents[id] != nil || len(h.agents) >= 4096 {
		return false
	}
	h.agents[id] = conn
	return true
}
func (h *liveHub) removeAgent(id string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.agents[id] == conn {
		delete(h.agents, id)
	}
}
func (a *App) agentStream(c *gin.Context) {
	// This is a native Agent endpoint. Browser clients use the cookie-authenticated /live route.
	if c.GetHeader("Origin") != "" {
		bad(c, 403, "Agent stream does not accept browser origins")
		return
	}
	conn, e := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if e != nil {
		return
	}
	defer conn.CloseNow()
	id, credential := c.GetString("server_id"), c.GetString("agent_hash")
	if !a.hub.addAgent(id, conn) {
		conn.Close(websocket.StatusTryAgainLater, "connection limit")
		return
	}
	defer a.hub.removeAgent(id, conn)
	conn.SetReadLimit(1 << 20)
	for {
		var msg agentMessage
		readCtx, cancel := context.WithTimeout(c.Request.Context(), 75*time.Second)
		e = wsjson.Read(readCtx, conn, &msg)
		cancel()
		if e != nil {
			return
		}
		if _, e = uuid.Parse(msg.RequestID); e != nil {
			conn.Close(websocket.StatusPolicyViolation, "invalid request id")
			return
		}
		ack := agentAck{RequestID: msg.RequestID, Type: "ack", Status: http.StatusOK}
		ctx, done := context.WithTimeout(c.Request.Context(), 20*time.Second)
		switch msg.Type {
		case "heartbeat":
			var in Heartbeat
			if json.Unmarshal(msg.Data, &in) != nil {
				e = errInvalidAgentMessage
			} else {
				e = a.recordHeartbeat(ctx, id, credential, in)
			}
			ack.Data = gin.H{"received": e == nil}
		case "events":
			var in EventBatch
			if json.Unmarshal(msg.Data, &in) != nil {
				e = errInvalidAgentMessage
			} else {
				ack.Data, e = a.ingest(ctx, id, credential, in)
			}
		default:
			e = errInvalidAgentMessage
		}
		done()
		switch {
		case errors.Is(e, errInvalidAgentMessage):
			ack.Status = http.StatusBadRequest
		case errors.Is(e, sql.ErrNoRows):
			ack.Status = http.StatusUnauthorized
		case e != nil:
			ack.Status = http.StatusServiceUnavailable
		}
		if ack.Status != http.StatusOK {
			ack.Data = nil
		}
		writeCtx, finish := context.WithTimeout(c.Request.Context(), 5*time.Second)
		e = wsjson.Write(writeCtx, conn, ack)
		finish()
		if e != nil || ack.Status == http.StatusUnauthorized {
			return
		}
	}
}
