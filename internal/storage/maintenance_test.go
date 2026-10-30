package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteSpaceReclamationPreservesRecords(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, "sqlite", filepath.Join(t.TempDir(), "maintenance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ctx, `CREATE TABLE payload(id integer PRIMARY KEY, body blob)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO payload VALUES(1,zeroblob(1048576)),(2,zeroblob(1024))`); err != nil {
		t.Fatal(err)
	}
	before, err := db.Usage(ctx)
	if err != nil || before <= 0 {
		t.Fatal(before, err)
	}
	if _, err = db.Exec(ctx, `DELETE FROM payload WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err = db.Reclaim(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := db.Usage(ctx)
	if err != nil || after >= before {
		t.Fatal("space not reclaimed", before, after, err)
	}
	var n int
	if err = db.Row(ctx, `SELECT length(body) FROM payload WHERE id=2`).Scan(&n); err != nil || n != 1024 {
		t.Fatal("retained record changed", n, err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err = db.Reclaim(cancelCtx); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err = (&DB{Driver: "unknown"}).Reclaim(ctx); err == nil {
		t.Fatal("accepted external maintenance")
	}
}
