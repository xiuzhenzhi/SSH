package api

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errLoginBlocked = errors.New("login source blocked")

func sourceIP(c *gin.Context) string {
	if ip := net.ParseIP(c.ClientIP()); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func (a *App) loginBlocked(ctx context.Context, ip string) (time.Time, error) {
	var until sql.NullTime
	e := a.db.Row(ctx, `SELECT blocked_until FROM login_guards WHERE source_hash=?`, hash(ip)).Scan(&until)
	if e == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if e != nil {
		return time.Time{}, e
	}
	if until.Valid && until.Time.After(time.Now()) {
		return until.Time, nil
	}
	return time.Time{}, nil
}

func (a *App) lockLoginGuard(ctx context.Context, tx *sql.Tx, ip string, now time.Time) (sql.NullTime, error) {
	q := `INSERT INTO login_guards(source_hash,source_ip,updated_at) VALUES(?,?,?) ON CONFLICT(source_hash) DO NOTHING`
	if a.db.Driver == "mysql" {
		q = `INSERT INTO login_guards(source_hash,source_ip,updated_at) VALUES(?,?,?) ON DUPLICATE KEY UPDATE source_hash=VALUES(source_hash)`
	}
	if _, e := tx.ExecContext(ctx, a.db.Bind(q), hash(ip), ip, now); e != nil {
		return sql.NullTime{}, e
	}
	var until sql.NullTime
	e := tx.QueryRowContext(ctx, a.db.Bind(`SELECT blocked_until FROM login_guards WHERE source_hash=? FOR UPDATE`), hash(ip)).Scan(&until)
	return until, e
}

// A per-source row serializes rolling-window failure counts across replicas.
func (a *App) recordLoginFailure(c *gin.Context, ip string) (time.Time, error) {
	ctx, now := c.Request.Context(), time.Now().UTC()
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return time.Time{}, e
	}
	defer tx.Rollback()
	until, e := a.lockLoginGuard(ctx, tx, ip, now)
	if e != nil {
		return time.Time{}, e
	}
	if until.Valid && until.Time.After(now) {
		return until.Time, nil
	}
	if _, e = tx.ExecContext(ctx, a.db.Bind(`DELETE FROM login_failures WHERE source_hash=? AND occurred_at<=?`), hash(ip), now.Add(-time.Minute)); e != nil {
		return time.Time{}, e
	}
	if _, e = tx.ExecContext(ctx, a.db.Bind(`INSERT INTO login_failures(id,source_hash,occurred_at) VALUES(?,?,?)`), uuid.NewString(), hash(ip), now); e != nil {
		return time.Time{}, e
	}
	var count int
	if e = tx.QueryRowContext(ctx, a.db.Bind(`SELECT COUNT(*) FROM login_failures WHERE source_hash=? AND occurred_at>?`), hash(ip), now.Add(-time.Minute)).Scan(&count); e != nil {
		return time.Time{}, e
	}
	var blocked any
	deadline := time.Time{}
	if count >= 5 {
		deadline = now.Add(24 * time.Hour)
		blocked = deadline
	}
	if _, e = tx.ExecContext(ctx, a.db.Bind(`UPDATE login_guards SET blocked_until=?,updated_at=? WHERE source_hash=?`), blocked, now, hash(ip)); e != nil {
		return time.Time{}, e
	}
	// Only the transition into a block is audited; failed passwords are never stored.
	if !deadline.IsZero() {
		if e = a.writeAudit(c, tx, nil, "auth.ip.block", ip); e != nil {
			return time.Time{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return time.Time{}, e
	}
	return deadline, nil
}

func denyBlocked(c *gin.Context, until time.Time) {
	seconds := max(1, int(time.Until(until).Seconds())+1)
	c.Header("Retry-After", strconv.Itoa(seconds))
	bad(c, 429, "该 IP 60 秒内登录错误达到 5 次，已封禁 24 小时")
}
func (a *App) failedLogin(c *gin.Context, ip, message string, status int) {
	until, e := a.recordLoginFailure(c, ip)
	if e != nil {
		fail(c, e)
		return
	}
	if !until.IsZero() {
		denyBlocked(c, until)
		return
	}
	bad(c, status, message)
}

func (a *App) blockedIPs(c *gin.Context) {
	a.list(c, `SELECT source_hash AS id,source_ip,blocked_until FROM login_guards WHERE blocked_until>? ORDER BY blocked_until DESC LIMIT ? OFFSET ?`, time.Now().UTC())
}
func (a *App) unblockIP(c *gin.Context) {
	e := a.mutate(c, c.MustGet("user").(User).ID, "auth.ip.unblock", c.Param("id"), func(tx *sql.Tx) error {
		var id string
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT source_hash FROM login_guards WHERE source_hash=? FOR UPDATE`), c.Param("id")).Scan(&id); e != nil {
			return e
		}
		if _, e := tx.ExecContext(c.Request.Context(), a.db.Bind(`DELETE FROM login_failures WHERE source_hash=?`), id); e != nil {
			return e
		}
		_, e := tx.ExecContext(c.Request.Context(), a.db.Bind(`UPDATE login_guards SET blocked_until=NULL,updated_at=? WHERE source_hash=?`), time.Now().UTC(), id)
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"unblocked": true})
}
