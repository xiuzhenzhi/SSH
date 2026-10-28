package api

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"sshguard/internal/storage"
)

func testRetentionPreservesUnsent(t *testing.T, db *storage.DB, serverID string) {
	t.Helper()
	ctx := context.Background()
	app := NewApp(db, Config{EventRetentionDays: 30, AlertRetentionDays: 90, AuditRetentionDays: 180})
	old := time.Now().UTC().AddDate(0, 0, -100)
	channelID := uuid.NewString()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO notification_channels(id,name,kind,config_ciphertext,enabled) VALUES(?,?,?,'fixture',false)`, channelID, "retention fixture", "webhook")
	ids := map[string]string{}
	for _, status := range []string{"sent", "pending", "dead"} {
		id := uuid.NewString()
		ids[status] = id
		exec(`INSERT INTO alerts(id,severity,title,message,created_at) VALUES(?,'info','retention fixture','fixture',?)`, id, old)
		exec(`INSERT INTO notification_jobs(id,alert_id,channel_id,status,next_attempt_at,created_at) VALUES(?,?,?,?,?,?)`, id, id, channelID, status, time.Now().UTC().Add(time.Hour), old)
	}
	eventID := uuid.NewString()
	exec(`INSERT INTO ssh_events(event_id,server_id,event_type,username,source_ip,outcome,occurred_at) VALUES(?,?,'ssh.login','retention-fixture','192.0.2.1','success',?)`, eventID, serverID, old)
	if e := app.cleanupHistory(ctx); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := db.Row(ctx, `SELECT COUNT(*) FROM ssh_events WHERE event_id=?`, eventID).Scan(&count); e != nil || count != 0 {
		t.Fatal("expired event retained", count, e)
	}
	for status, id := range ids {
		expected := 1
		if status == "sent" {
			expected = 0
		}
		if e := db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE id=?`, id).Scan(&count); e != nil || count != expected {
			t.Fatal("retention erased unsent evidence", status, count, e)
		}
		if e := db.Row(ctx, `SELECT COUNT(*) FROM notification_jobs WHERE id=?`, id).Scan(&count); e != nil || count != expected {
			t.Fatal("retention erased unsent job", status, count, e)
		}
		exec(`DELETE FROM notification_jobs WHERE id=?`, id)
		exec(`DELETE FROM alerts WHERE id=?`, id)
	}
	exec(`DELETE FROM notification_channels WHERE id=?`, channelID)
}
