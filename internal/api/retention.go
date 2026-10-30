package api

import (
	"context"
	"log"
	"strings"
	"time"
)

// Only sent notifications are removed automatically. Pending, in-flight and dead
// jobs retain their alert so a delivery outage cannot erase its own evidence.
func (a *App) cleanupHistory(ctx context.Context) error {
	settings, err := a.readRetention(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, item := range []struct {
		table, where string
		days         int
	}{
		{"ssh_events", "occurred_at<?", settings.Events},
		{"audit_logs", "created_at<?", settings.Audit},
		{"notification_jobs", "created_at<? AND status='sent'", settings.Alerts},
		{"alerts", "created_at<? AND NOT EXISTS (SELECT 1 FROM notification_jobs j WHERE j.alert_id=alerts.id)", settings.Alerts},
		{"alert_state", "last_alert_at<?", max(settings.Alerts, 30)},
	} {
		if item.days < 1 {
			continue
		}
		key := "id"
		if item.table == "alert_state" {
			key = "state_key"
		}
		cutoff := now.AddDate(0, 0, -item.days)
		rows, e := a.db.QueryContext(ctx, a.db.Bind("SELECT "+key+" FROM "+item.table+" WHERE "+item.where+" LIMIT 1000"), cutoff)
		if e != nil {
			return e
		}
		ids := []any{}
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
		if len(ids) == 0 {
			continue
		}
		args := append([]any{cutoff}, ids...)
		// Recheck mutable status/state at deletion time; identifiers are fixed in source.
		query := "DELETE FROM " + item.table + " WHERE " + item.where + " AND " + key + " IN (" + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + ")"
		if _, e = a.db.Exec(ctx, query, args...); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) retentionLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		work, cancel := context.WithTimeout(ctx, 20*time.Second)
		if e := a.cleanupHistory(work); e != nil && ctx.Err() == nil {
			log.Print("History retention cleanup failed")
		}
		cancel()
	}
}
