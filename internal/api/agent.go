package api

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net"
	"strings"
	"time"
)

func (a *App) enroll(c *gin.Context) {
	var in struct {
		Token        string `json:"token" binding:"required,len=64"`
		Hostname     string `json:"hostname" binding:"required,max=253"`
		Distribution string `json:"distribution" binding:"max=128"`
		Version      string `json:"version" binding:"required,max=32"`
	}
	if !bind(c, &in) {
		return
	}
	t := token()
	var id string
	// Lock server first for a consistent lock order with revocation and token replacement.
	if e := a.db.Row(c, `SELECT server_id FROM agent_enrollments WHERE token_hash=?`, hash(in.Token)).Scan(&id); e != nil {
		if e == sql.ErrNoRows {
			bad(c, 401, "注册凭证无效")
		} else {
			fail(c, e)
		}
		return
	}
	e := a.mutate(c, "", "agent.enrolled", id, func(tx *sql.Tx) error {
		var locked string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM servers WHERE id=? FOR UPDATE`), id).Scan(&locked); e != nil {
			return e
		}
		var enrolled string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT server_id FROM agent_enrollments WHERE token_hash=? AND used_at IS NULL AND expires_at>? FOR UPDATE`), hash(in.Token), time.Now().UTC()).Scan(&enrolled); e != nil {
			return e
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`UPDATE agent_enrollments SET used_at=? WHERE token_hash=?`), time.Now().UTC(), hash(in.Token)); e != nil {
			return e
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`UPDATE agent_credentials SET revoked_at=CURRENT_TIMESTAMP WHERE server_id=?`), id); e != nil {
			return e
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO agent_credentials(token_hash,server_id) VALUES(?,?)`), hash(t), id); e != nil {
			return e
		}
		_, e := tx.ExecContext(c, a.db.Bind(`UPDATE servers SET hostname=?,distribution=?,agent_version=?,last_seen_at=?,revoked=false WHERE id=?`), in.Hostname, in.Distribution, in.Version, time.Now().UTC(), id)
		return e
	})
	if e != nil {
		if e == sql.ErrNoRows {
			bad(c, 401, "注册凭证过期或已使用")
		} else {
			fail(c, e)
		}
		return
	}
	ok(c, gin.H{"server_id": id, "token": t})
}
func (a *App) agentAuth(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || len(auth) != 71 {
		bad(c, 401, "Agent 凭证无效")
		return
	}
	h := hash(strings.TrimPrefix(auth, "Bearer "))
	var id string
	e := a.db.Row(c, `SELECT s.id FROM agent_credentials ac JOIN servers s ON s.id=ac.server_id WHERE ac.token_hash=? AND ac.revoked_at IS NULL AND s.revoked=false`, h).Scan(&id)
	if e == sql.ErrNoRows {
		bad(c, 401, "Agent 凭证无效")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	c.Set("server_id", id)
	c.Set("agent_hash", h)
	c.Next()
}
func (a *App) agentTx(c *gin.Context, fn func(*sql.Tx) error) error {
	tx, e := a.db.BeginTx(c, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var id string
	if e = tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM servers WHERE id=? AND revoked=false FOR UPDATE`), c.GetString("server_id")).Scan(&id); e != nil {
		return e
	}
	if e = tx.QueryRowContext(c, a.db.Bind(`SELECT server_id FROM agent_credentials WHERE token_hash=? AND revoked_at IS NULL`), c.GetString("agent_hash")).Scan(&id); e != nil {
		return e
	}
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
func (a *App) heartbeat(c *gin.Context) {
	var in struct {
		Version string `json:"version" binding:"max=32"`
	}
	if !bind(c, &in) {
		return
	}
	e := a.agentTx(c, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(c, a.db.Bind(`UPDATE servers SET last_seen_at=?,agent_version=CASE WHEN ?='' THEN agent_version ELSE ? END WHERE id=?`), time.Now().UTC(), in.Version, in.Version, c.GetString("server_id"))
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"received": true, "collector": "not-implemented"})
}

type Event struct {
	ID       string    `json:"event_id" binding:"required,uuid"`
	Type     string    `json:"event_type" binding:"required,oneof=ssh.login ssh.login_failed"`
	Username string    `json:"username" binding:"required,max=128"`
	IP       string    `json:"source_ip" binding:"required,ip"`
	Port     *int      `json:"source_port" binding:"omitempty,min=1,max=65535"`
	Method   string    `json:"auth_method" binding:"max=32"`
	Outcome  string    `json:"outcome" binding:"required,oneof=success failure"`
	At       time.Time `json:"occurred_at" binding:"required"`
}

func validEvent(e Event, now time.Time) bool {
	return !e.At.Before(now.AddDate(0, 0, -30)) && !e.At.After(now.Add(5*time.Minute)) && ((e.Type == "ssh.login" && e.Outcome == "success") || (e.Type == "ssh.login_failed" && e.Outcome == "failure")) && !strings.ContainsAny(e.Username, "\r\n\x00")
}
func (a *App) batch(c *gin.Context) {
	var in struct {
		Events []Event `json:"events" binding:"required,min=1,max=100,dive"`
	}
	if !bind(c, &in) {
		return
	}
	for i, e := range in.Events {
		if !validEvent(e, time.Now().UTC()) {
			bad(c, 400, "事件时间或类型不合法，允许最近 30 天及最多 5 分钟时钟偏差")
			return
		}
		in.Events[i].At = e.At.UTC()
		in.Events[i].ID = uuid.MustParse(e.ID).String()
		in.Events[i].IP = net.ParseIP(e.IP).String()
	}
	inserted := 0
	e := a.agentTx(c, func(tx *sql.Tx) error {
		for _, event := range in.Events {
			// Server row lock serializes same-server batches on both databases.
			var n int
			if e := tx.QueryRowContext(c, a.db.Bind(`SELECT COUNT(*) FROM ssh_events WHERE server_id=? AND event_id=?`), c.GetString("server_id"), event.ID).Scan(&n); e != nil {
				return e
			}
			if n > 0 {
				continue
			}
			_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO ssh_events(event_id,server_id,event_type,username,source_ip,source_port,auth_method,outcome,occurred_at) VALUES(?,?,?,?,?,?,?,?,?)`), event.ID, c.GetString("server_id"), event.Type, event.Username, event.IP, event.Port, event.Method, event.Outcome, event.At)
			if e != nil {
				return e
			}
			inserted++
		}
		return nil
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"accepted": len(in.Events), "inserted": inserted, "duplicates": len(in.Events) - inserted})
}
