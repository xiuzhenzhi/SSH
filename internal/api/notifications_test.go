package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"sshguard/internal/notify"
	"sshguard/internal/storage"
)

func testDeliveryLeases(t *testing.T, db *storage.DB) {
	t.Helper()
	ctx := context.Background()
	app := NewApp(db, Config{EncryptionKey: strings.Repeat("ab", 32)})
	channelID, alertID, jobID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	raw, _ := json.Marshal(notify.Config{URL: "https://example.com/fixture"})
	encrypted, e := notify.Seal(app.cfg.EncryptionKey, channelID, raw)
	if e != nil {
		t.Fatal(e)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO notification_channels(id,name,kind,config_ciphertext) VALUES(?,?,?,?)`, channelID, "lease fixture", "webhook", encrypted)
	exec(`INSERT INTO alerts(id,severity,title,message) VALUES(?,?,?,?)`, alertID, "info", "test", "local test only")
	exec(`INSERT INTO notification_jobs(id,alert_id,channel_id,next_attempt_at) VALUES(?,?,?,?)`, jobID, alertID, channelID, time.Now().UTC().Add(-time.Minute))
	first, e := app.claimDelivery(ctx)
	if e != nil || first == nil || first.id != jobID || first.attempts != 1 {
		t.Fatal("initial claim", first, e)
	}
	second, e := app.claimDelivery(ctx)
	if e != nil || second != nil {
		t.Fatal("claimed an active lease twice", e)
	}
	exec(`UPDATE notification_jobs SET locked_until=? WHERE id=?`, time.Now().UTC().Add(-time.Minute), jobID)
	exec(`UPDATE notification_channels SET last_attempt_at=NULL WHERE id=?`, channelID)
	second, e = app.claimDelivery(ctx)
	if e != nil || second == nil || second.lease == first.lease || second.attempts != 2 {
		t.Fatal("expired lease recovery", e)
	}
	if e = app.finishDelivery(ctx, first, nil); e != nil {
		t.Fatal(e)
	}
	var status string
	if e = db.Row(ctx, `SELECT status FROM notification_jobs WHERE id=?`, jobID).Scan(&status); e != nil || status != "sending" {
		t.Fatal("stale worker overwrote current lease", status, e)
	}
	if e = app.finishDelivery(ctx, second, errors.New("provider secret must not be stored")); e != nil {
		t.Fatal(e)
	}
	var message string
	var next time.Time
	if e = db.Row(ctx, `SELECT status,last_error,next_attempt_at FROM notification_jobs WHERE id=?`, jobID).Scan(&status, &message, &next); e != nil || status != "pending" || strings.Contains(message, "secret") || next.Before(time.Now().UTC().Add(50*time.Second)) {
		t.Fatal("retry policy", status, message, next, e)
	}
	exec(`UPDATE notification_jobs SET next_attempt_at=? WHERE id=?`, time.Now().UTC().Add(-time.Minute), jobID)
	exec(`UPDATE notification_channels SET last_attempt_at=NULL WHERE id=?`, channelID)
	third, e := app.claimDelivery(ctx)
	if e != nil || third == nil {
		t.Fatal("retry claim", e)
	}
	if e = app.finishDelivery(ctx, third, nil); e != nil {
		t.Fatal(e)
	}
	if e = db.Row(ctx, `SELECT status FROM notification_jobs WHERE id=?`, jobID).Scan(&status); e != nil || status != "sent" {
		t.Fatal("successful delivery not persisted", e)
	}
	// Do not leave an enabled fixture channel that subsequent test events can target.
	exec(`UPDATE notification_channels SET enabled=false WHERE id=?`, channelID)
}
