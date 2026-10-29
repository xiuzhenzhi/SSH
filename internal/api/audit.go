package api

import (
	"database/sql"
	"net"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

func (a *App) writeAudit(c *gin.Context, tx *sql.Tx, actor any, action, target string) error {
	query := `INSERT INTO audit_logs(actor_id,action,target) VALUES(?,?,?)`
	var id int64
	if a.db.Driver == "postgres" {
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(query+` RETURNING id`), actor, action, target).Scan(&id); e != nil {
			return e
		}
	} else {
		result, e := tx.ExecContext(c.Request.Context(), a.db.Bind(query), actor, action, target)
		if e != nil {
			return e
		}
		if id, e = result.LastInsertId(); e != nil {
			return e
		}
	}
	ip := ""
	if parsed := net.ParseIP(c.ClientIP()); parsed != nil {
		ip = parsed.String()
	}
	ua := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, c.Request.UserAgent())
	runes := []rune(ua)
	if len(runes) > 512 {
		ua = string(runes[:512])
	}
	_, e := tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO audit_context(audit_id,source_ip,user_agent) VALUES(?,?,?)`), id, ip, ua)
	return e
}
