package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

type trendBucket struct {
	At      time.Time `json:"at"`
	Success int64     `json:"success"`
	Failure int64     `json:"failure"`
}

func (a *App) overview(c *gin.Context) {
	now := time.Now().UTC()
	since := now.Truncate(time.Hour).Add(-23 * time.Hour)
	buckets := make([]trendBucket, 24)
	for i := range buckets {
		buckets[i].At = since.Add(time.Duration(i) * time.Hour)
	}
	expression := `FLOOR(EXTRACT(EPOCH FROM occurred_at)/3600)`
	if a.db.Driver == "mysql" {
		expression = `FLOOR(UNIX_TIMESTAMP(occurred_at)/3600)`
	}
	rows, e := a.db.QueryContext(c, a.db.Bind(`SELECT `+expression+`,SUM(CASE WHEN outcome='success' THEN 1 ELSE 0 END),SUM(CASE WHEN outcome='failure' THEN 1 ELSE 0 END) FROM ssh_events WHERE occurred_at>=? AND occurred_at<=? GROUP BY `+expression+` ORDER BY 1`), since, now)
	if e != nil {
		fail(c, e)
		return
	}
	for rows.Next() {
		var hour, success, failure int64
		if e = rows.Scan(&hour, &success, &failure); e != nil {
			rows.Close()
			fail(c, e)
			return
		}
		i := hour - since.Unix()/3600
		if i >= 0 && i < 24 {
			buckets[i].Success = success
			buckets[i].Failure = failure
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		fail(c, e)
		return
	}
	var healthy, attention, pending, dead, channels int64
	if e = a.db.Row(c, `SELECT COUNT(*) FROM agent_health h JOIN servers s ON s.id=h.server_id WHERE s.revoked=false AND s.last_seen_at>? AND h.collector_state='healthy' AND h.updated_at>? AND h.last_poll_at>?`, now.Add(-90*time.Second), now.Add(-90*time.Second), now.Add(-90*time.Second)).Scan(&healthy); e != nil {
		fail(c, e)
		return
	}
	if e = a.db.Row(c, `SELECT COUNT(*) FROM alerts WHERE acknowledged_at IS NULL AND resolved_at IS NULL`).Scan(&attention); e != nil {
		fail(c, e)
		return
	}
	if e = a.db.Row(c, `SELECT COALESCE(SUM(CASE WHEN status='pending' OR status='sending' THEN 1 ELSE 0 END),0),COALESCE(SUM(CASE WHEN status='dead' THEN 1 ELSE 0 END),0) FROM notification_jobs`).Scan(&pending, &dead); e != nil {
		fail(c, e)
		return
	}
	if e = a.db.Row(c, `SELECT COUNT(*) FROM notification_channels WHERE enabled=true`).Scan(&channels); e != nil {
		fail(c, e)
		return
	}
	alerts, e := a.db.List(c, `SELECT a.id,a.title,a.severity,a.created_at,a.acknowledged_at,a.resolved_at,s.name AS server_name FROM alerts a LEFT JOIN servers s ON s.id=a.server_id ORDER BY a.created_at DESC LIMIT 5`)
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"trend": buckets, "healthy_collectors": healthy, "unacknowledged_alerts": attention, "pending_deliveries": pending, "failed_deliveries": dead, "enabled_channels": channels, "recent_alerts": alerts, "as_of": now})
}
