package api

import (
	"context"
	"testing"
	"time"

	"sshguard/internal/storage"
)

func testHealthRecovery(t *testing.T, db *storage.DB, serverID, credential string) {
	t.Helper()
	ctx := context.Background()
	app := NewApp(db, Config{})
	now := time.Now().UTC()
	if e := app.recordHeartbeat(ctx, serverID, hash(credential), Heartbeat{State: "error", Message: "fixture unreadable source", LastPoll: &now}); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, `UPDATE agent_health SET state_since=? WHERE server_id=?`, now.Add(-5*time.Minute), serverID); e != nil {
		t.Fatal(e)
	}
	rules, e := app.readRules(ctx, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = app.monitorServer(ctx, serverID, rules); e != nil {
		t.Fatal(e)
	}
	var count int
	collectorRule := "00000000-0000-4000-8000-000000000004"
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=? AND rule_id=? AND resolved_at IS NULL`, serverID, collectorRule).Scan(&count); e != nil || count != 1 {
		t.Fatal("collector incident missing", count, e)
	}
	if e = app.monitorServer(ctx, serverID, rules); e != nil {
		t.Fatal(e)
	}
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=? AND rule_id=?`, serverID, collectorRule).Scan(&count); e != nil || count != 1 {
		t.Fatal("health cooldown failed", count, e)
	}
	if e = app.recordHeartbeat(ctx, serverID, hash(credential), Heartbeat{State: "healthy", LastPoll: &now}); e != nil {
		t.Fatal(e)
	}
	if e = app.monitorServer(ctx, serverID, rules); e != nil {
		t.Fatal(e)
	}
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=? AND rule_id=? AND resolved_at IS NOT NULL`, serverID, collectorRule).Scan(&count); e != nil || count != 1 {
		t.Fatal("health recovery not recorded", count, e)
	}
	if _, e = db.Exec(ctx, `UPDATE servers SET last_seen_at=? WHERE id=?`, now.Add(-5*time.Minute), serverID); e != nil {
		t.Fatal(e)
	}
	if e = app.monitorServer(ctx, serverID, rules); e != nil {
		t.Fatal(e)
	}
	offlineRule := "00000000-0000-4000-8000-000000000003"
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=? AND rule_id=? AND resolved_at IS NULL`, serverID, offlineRule).Scan(&count); e != nil || count != 1 {
		t.Fatal("offline incident missing", count, e)
	}
	if e = app.recordHeartbeat(ctx, serverID, hash(credential), Heartbeat{State: "healthy", LastPoll: &now}); e != nil {
		t.Fatal(e)
	}
	if e = app.monitorServer(ctx, serverID, rules); e != nil {
		t.Fatal(e)
	}
	if e = db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=? AND rule_id=? AND resolved_at IS NOT NULL`, serverID, offlineRule).Scan(&count); e != nil || count != 1 {
		t.Fatal("offline recovery missing", count, e)
	}
}
