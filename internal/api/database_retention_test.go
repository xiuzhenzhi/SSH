package api

import (
	"context"
	"path/filepath"
	"sshguard/internal/storage"
	"testing"
	"time"
)

func TestSavedRetentionControlsCleanupAndPermanentRetention(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	app := NewApp(db, Config{AuditRetentionDays: 180})
	defer app.Shutdown()
	old := time.Now().UTC().AddDate(0, 0, -2)
	if _, err = db.Exec(ctx, `INSERT INTO retention_settings VALUES(1,0,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO audit_logs(action,target,created_at) VALUES('fixture','test',?)`, old); err != nil {
		t.Fatal(err)
	}
	if err = app.cleanupHistory(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.Row(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil || count != 1 {
		t.Fatal("permanent record deleted", err)
	}
	if _, err = db.Exec(ctx, `UPDATE retention_settings SET audit_days=1`); err != nil {
		t.Fatal(err)
	}
	if err = app.cleanupHistory(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.Row(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil || count != 0 {
		t.Fatal("saved retention ignored", err)
	}
}

func TestMaintenanceStopWaitsForFinalCleanup(t *testing.T) {
	m := databaseMaintenance{done: make(chan struct{})}
	stopped := make(chan struct{})
	go func() { m.stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("shutdown returned before maintenance cleanup finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(m.done)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
}
