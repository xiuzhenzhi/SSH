package storage

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSQLitePersistenceAndConstraints(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "guard.db")
	db, err := Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { db.Close() }()
	if err = Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var journal string
	if err = db.Row(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil || journal != "wal" {
		t.Fatalf("journal=%s: %v", journal, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES('orphan','missing',?)`, time.Now().UTC()); err == nil {
		t.Fatal("foreign key not enforced")
	}
	if _, err = db.Exec(ctx, `INSERT INTO users(id,username,password_hash,role) VALUES('user','admin','hash','admin')`); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	if _, err = db.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES('live','user',?),('expired','user',?)`, expires, expires.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = Cleanup(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	var got time.Time
	if err = db.Row(ctx, `SELECT expires_at FROM sessions WHERE token_hash='live'`).Scan(&got); err != nil || !got.Equal(expires) {
		t.Fatalf("timestamp roundtrip: %v %v", got, err)
	}
	var count int
	if err = db.Row(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cleanup: %d %v", count, err)
	}
}

func TestSQLiteConcurrentMigrationsAndTransactions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "guard.db")
	a, err := Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, "sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var wg sync.WaitGroup
	for _, db := range []*DB{a, b} {
		wg.Add(1)
		go func(db *DB) {
			defer wg.Done()
			if err := Migrate(ctx, db); err != nil {
				t.Error(err)
			}
		}(db)
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	if _, err = a.Exec(ctx, `INSERT INTO login_attempts(bucket_key,attempts,window_start) VALUES('counter',0,?)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// Two independent pools must serialize read-modify-write transactions.
	for i := 0; i < 10; i++ {
		for _, db := range []*DB{a, b} {
			wg.Add(1)
			go func(db *DB) {
				defer wg.Done()
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer tx.Rollback()
				var n int
				if err = tx.QueryRowContext(ctx, db.Bind(`SELECT attempts FROM login_attempts WHERE bucket_key='counter' FOR UPDATE`)).Scan(&n); err != nil {
					t.Error(err)
					return
				}
				if _, err = tx.ExecContext(ctx, `UPDATE login_attempts SET attempts=? WHERE bucket_key='counter'`, n+1); err != nil {
					t.Error(err)
					return
				}
				if err = tx.Commit(); err != nil {
					t.Error(err)
				}
			}(db)
		}
	}
	wg.Wait()
	var n int
	if err = a.Row(ctx, `SELECT attempts FROM login_attempts WHERE bucket_key='counter'`).Scan(&n); err != nil || n != 20 {
		t.Fatalf("lost updates: %d %v", n, err)
	}
}
