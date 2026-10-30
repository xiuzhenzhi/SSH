package api

import (
	"context"
	"github.com/google/uuid"
	"net/http/httptest"
	"path/filepath"
	"sshguard/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestAuditRegressions(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(ctx, "sqlite", filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	app := NewApp(db, Config{PublicURL: "https://guard.example.com"})
	defer app.Shutdown()
	uid, session := uuid.NewString(), token()
	if _, err := db.Exec(ctx, `INSERT INTO users(id,username,password_hash,role) VALUES(?,?,?,'admin')`, uid, "audit", "unused"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)`, hash(session), uid, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	router := app.Router()
	call := func(ip, path, cookie string) int {
		req := httptest.NewRequest("POST", path, strings.NewReader(`{}`))
		req.RemoteAddr = ip + ":1234"
		req.Header.Set("Origin", "https://guard.example.com")
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.Header.Set("Cookie", "sshguard_session="+cookie)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res.Code
	}
	for i := 0; i < 60; i++ {
		if got := call("192.0.2.1", "/api/v1/auth/login", ""); got != 400 {
			t.Fatalf("attempt %d: %d", i, got)
		}
	}
	if got := call("192.0.2.1", "/api/v1/auth/login", ""); got != 429 {
		t.Fatal(got)
	}
	if got := call("192.0.2.2", "/api/v1/auth/login", ""); got != 400 {
		t.Fatalf("other source blocked: %d", got)
	}
	if got := call("192.0.2.1", "/api/v1/auth/password", session); got != 400 {
		t.Fatalf("authenticated action blocked: %d", got)
	}
	for _, tc := range []struct {
		name     string
		offsets  []int
		separate bool
		want     int
	}{
		{"reverse batch", []int{20, 10, 0}, false, 1},
		{"late across batches", []int{20, 10, 0}, true, 1},
		{"outside window", []int{80, 40, 0}, true, 0},
		{"chronological", []int{0, 10, 20}, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sid, credential, rid := uuid.NewString(), token(), uuid.NewString()
			if _, err := db.Exec(ctx, `INSERT INTO servers(id,name,group_name) VALUES(?,?,?)`, sid, tc.name, sid); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `INSERT INTO agent_credentials(token_hash,server_id) VALUES(?,?)`, hash(credential), sid); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(ctx, `INSERT INTO alert_rules(id,name,kind,enabled,threshold_count,window_seconds,cooldown_seconds,server_group,trusted_cidrs,channel_ids) VALUES(?,?,'failure_threshold',true,3,60,60,?,'[]','[]')`, rid, tc.name, sid); err != nil {
				t.Fatal(err)
			}
			events := []Event{}
			base := time.Now().UTC().Add(-time.Hour)
			for _, offset := range tc.offsets {
				events = append(events, Event{ID: uuid.NewString(), Type: "ssh.login_failed", Username: "audit", IP: "192.0.2.10", Outcome: "failure", At: base.Add(time.Duration(offset) * time.Second)})
			}
			if tc.separate {
				for _, event := range events {
					if _, err := app.ingest(ctx, sid, hash(credential), EventBatch{Events: []Event{event}}); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				if _, err := app.ingest(ctx, sid, hash(credential), EventBatch{Events: events}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := app.ingest(ctx, sid, hash(credential), EventBatch{Events: events}); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.Row(ctx, `SELECT COUNT(*) FROM alerts WHERE server_id=?`, sid).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != tc.want {
				t.Fatalf("alerts=%d want=%d", count, tc.want)
			}
		})
	}
}
