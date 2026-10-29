package api

import (
	"github.com/gin-gonic/gin"
	"net"
	"time"
)

func (a *App) events(c *gin.Context) {
	q := `SELECT e.*,s.name AS server_name FROM ssh_events e JOIN servers s ON s.id=e.server_id WHERE 1=1`
	args := []any{}
	for _, f := range []struct{ key, col string }{{"server_id", "e.server_id"}, {"source_ip", "e.source_ip"}, {"username", "e.username"}, {"outcome", "e.outcome"}, {"event_type", "e.event_type"}} {
		if v := c.Query(f.key); v != "" {
			if f.key == "source_ip" {
				ip := net.ParseIP(v)
				if ip == nil {
					bad(c, 400, "IP 地址无效")
					return
				}
				v = ip.String()
			}
			q += " AND " + f.col + "=?"
			args = append(args, v)
		}
	}
	for _, f := range []struct{ key, op string }{{"from", ">="}, {"to", "<="}} {
		if v := c.Query(f.key); v != "" {
			t, e := time.Parse(time.RFC3339, v)
			if e != nil {
				bad(c, 400, "时间须为 RFC3339 格式")
				return
			}
			q += " AND e.occurred_at" + f.op + "?"
			args = append(args, t.UTC())
		}
	}
	a.list(c, q+` ORDER BY e.occurred_at DESC,e.id DESC LIMIT ? OFFSET ?`, args...)
}
func (a *App) event(c *gin.Context) {
	rows, e := a.db.List(c.Request.Context(), `SELECT e.*,s.name AS server_name FROM ssh_events e JOIN servers s ON e.server_id=s.id WHERE e.id=?`, c.Param("id"))
	if e != nil {
		fail(c, e)
		return
	}
	if len(rows) == 0 {
		bad(c, 404, "事件不存在")
		return
	}
	ok(c, rows[0])
}
