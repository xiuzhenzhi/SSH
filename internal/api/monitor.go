package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

func (a *App) monitorAgents(ctx context.Context) error {
	rules, e := a.readRules(ctx, true)
	if e != nil {
		return e
	}
	rows, e := a.db.QueryContext(ctx, `SELECT id FROM servers WHERE revoked=false AND last_seen_at IS NOT NULL ORDER BY id`)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = a.monitorServer(ctx, id, rules); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) monitorServer(ctx context.Context, id string, rules []Rule) error {
	tx, e := a.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var name, group string
	var seen, created time.Time
	if e = tx.QueryRowContext(ctx, a.db.Bind(`SELECT name,group_name,last_seen_at,created_at FROM servers WHERE id=? AND revoked=false FOR UPDATE`), id).Scan(&name, &group, &seen, &created); e == sql.ErrNoRows {
		return nil
	} else if e != nil {
		return e
	}
	var state, message string
	var since time.Time
	e = tx.QueryRowContext(ctx, a.db.Bind(`SELECT collector_state,collector_message,state_since FROM agent_health WHERE server_id=?`), id).Scan(&state, &message, &since)
	if e == sql.ErrNoRows {
		state = "unknown"
		since = created
	} else if e != nil {
		return e
	}
	now := time.Now().UTC()
	for _, rule := range rules {
		if rule.Group != "" && rule.Group != group {
			continue
		}
		active := false
		text := ""
		switch rule.Kind {
		case "agent_offline":
			active = now.Sub(seen) > time.Duration(rule.Threshold)*time.Second
			text = fmt.Sprintf("服务器：%s\n最后心跳：%s\n请检查主机、网络及 Agent 服务。", name, seen.Format(time.RFC3339))
		case "collector_error":
			// Offline has its own rule; do not repeatedly report stale collector state.
			active = now.Sub(seen) <= 90*time.Second && state != "healthy" && now.Sub(since) > time.Duration(rule.Threshold)*time.Second
			text = fmt.Sprintf("服务器：%s\n采集状态：%s\n诊断：%s", name, state, message)
		default:
			continue
		}
		if active {
			if e = a.raiseAlert(ctx, tx, rule, id, "", "health", "high", text); e != nil {
				return e
			}
		} else if (rule.Kind == "agent_offline" && now.Sub(seen) <= 90*time.Second) || (rule.Kind == "collector_error" && state == "healthy" && now.Sub(seen) <= 90*time.Second) {
			if _, e = tx.ExecContext(ctx, a.db.Bind(`UPDATE alerts SET resolved_at=? WHERE server_id=? AND rule_id=? AND resolved_at IS NULL`), now, id, rule.ID); e != nil {
				return e
			}
		}
	}
	if e = tx.Commit(); e == nil {
		a.hub.publish()
	}
	return e
}
func (a *App) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, 20*time.Second)
		if e := a.monitorAgents(work); e != nil && ctx.Err() == nil {
			log.Print("Agent health monitoring failed")
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
