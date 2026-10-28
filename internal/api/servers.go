package api

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strings"
	"time"
)

const serverColumns = `id,name,hostname,distribution,group_name,notes,agent_version,last_seen_at,created_at,CASE WHEN revoked=true THEN 'revoked' WHEN last_seen_at IS NULL THEN 'pending' WHEN last_seen_at>? THEN 'online' ELSE 'offline' END AS status,(SELECT collector_state FROM agent_health WHERE server_id=servers.id) AS collector_state,(SELECT collector_message FROM agent_health WHERE server_id=servers.id) AS collector_message,(SELECT queue_depth FROM agent_health WHERE server_id=servers.id) AS queue_depth,(SELECT rejected_events FROM agent_health WHERE server_id=servers.id) AS rejected_events`

func (a *App) servers(c *gin.Context) {
	q := "%" + c.Query("q") + "%"
	a.list(c, `SELECT `+serverColumns+` FROM servers WHERE name LIKE ? OR hostname LIKE ? OR group_name LIKE ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, time.Now().UTC().Add(-90*time.Second), q, q, q)
}
func (a *App) server(c *gin.Context) {
	rows, e := a.db.List(c, `SELECT `+serverColumns+` FROM servers WHERE id=?`, time.Now().UTC().Add(-90*time.Second), c.Param("id"))
	if e != nil {
		fail(c, e)
		return
	}
	if len(rows) == 0 {
		bad(c, 404, "服务器不存在")
		return
	}
	ok(c, rows[0])
}
func (a *App) saveServer(c *gin.Context) {
	var in struct {
		Name  string `json:"name" binding:"required,max=128"`
		Group string `json:"group_name" binding:"max=64"`
		Notes string `json:"notes" binding:"max=1024"`
	}
	if !bind(c, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		bad(c, 400, "名称不能为空")
		return
	}
	id := c.Param("id")
	action := "server.update"
	create := id == ""
	if create {
		id = uuid.NewString()
		action = "server.create"
	}
	e := a.mutate(c, c.MustGet("user").(User).ID, action, id, func(tx *sql.Tx) error {
		if create {
			_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO servers(id,name,group_name,notes) VALUES(?,?,?,?)`), id, in.Name, in.Group, in.Notes)
			return e
		}
		var exists string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM servers WHERE id=? FOR UPDATE`), id).Scan(&exists); e != nil {
			return e
		}
		_, e := tx.ExecContext(c, a.db.Bind(`UPDATE servers SET name=?,group_name=?,notes=? WHERE id=?`), in.Name, in.Group, in.Notes, id)
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"id": id})
}
func shellQuote(s string) string      { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func powershellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func installCommands(origin, t string) map[string]string {
	linux := func(privilege string) string {
		return fmt.Sprintf(`(f=$(mktemp) && trap 'rm -f "$f"' EXIT && curl --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 60 -fsS %s -o "$f" && %ssh "$f" %s %s)`, shellQuote(origin+"/downloads/install-agent.sh"), privilege, shellQuote(origin), shellQuote(t))
	}
	windows := fmt.Sprintf(`[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; $f=Join-Path $env:TEMP ([guid]::NewGuid().ToString()+'.ps1'); try { Invoke-WebRequest -UseBasicParsing -TimeoutSec 60 -MaximumRedirection 0 -Uri %s -OutFile $f; if (-not $?) { throw 'Installer download failed' }; & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $f -Endpoint %s -EnrollmentToken %s; if ($LASTEXITCODE -ne 0) { throw 'Agent installation/update failed' } } finally { if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f } }`, powershellQuote(origin+"/downloads/install-agent.ps1"), powershellQuote(origin), powershellQuote(t))
	return map[string]string{"ubuntu": linux("sudo "), "debian": linux("sudo "), "alpine": linux(""), "windows": windows}
}
func (a *App) enrollment(c *gin.Context) {
	id := c.Param("id")
	t := token()
	expires := time.Now().UTC().Add(15 * time.Minute)
	e := a.mutate(c, c.MustGet("user").(User).ID, "agent.enrollment.create", id, func(tx *sql.Tx) error {
		var existing string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM servers WHERE id=? FOR UPDATE`), id).Scan(&existing); e != nil {
			return e
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`DELETE FROM agent_enrollments WHERE server_id=?`), id); e != nil {
			return e
		}
		_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO agent_enrollments(token_hash,server_id,expires_at) VALUES(?,?,?)`), hash(t), id, expires)
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	commands := installCommands(a.cfg.PublicURL, t)
	ok(c, gin.H{"command": commands["ubuntu"], "commands": commands, "expires_at": expires, "phase": "registration-and-heartbeat-only"})
}
func updateCommands(origin string) map[string]string {
	commands := installCommands(origin, "--update")
	commands["windows"] = strings.Replace(commands["windows"], "-EnrollmentToken '--update'", "-Update", 1)
	return commands
}
func (a *App) revoke(c *gin.Context) {
	id := c.Param("id")
	e := a.mutate(c, c.MustGet("user").(User).ID, "agent.revoke", id, func(tx *sql.Tx) error {
		var existing string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM servers WHERE id=? FOR UPDATE`), id).Scan(&existing); e != nil {
			return e
		}
		for _, q := range []string{`UPDATE servers SET revoked=true WHERE id=?`, `UPDATE agent_credentials SET revoked_at=CURRENT_TIMESTAMP WHERE server_id=?`, `DELETE FROM agent_enrollments WHERE server_id=?`} {
			if _, e := tx.ExecContext(c, a.db.Bind(q), id); e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"revoked": true})
}
