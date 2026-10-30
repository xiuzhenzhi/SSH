package api

import (
	"context"
	"net/http/httptest"
	"sshguard/internal/storage"
	"testing"
	"time"
)

func testDatabaseSettings(t *testing.T, db *storage.DB, session string, runningApp *App, call func(string, string, any, string, string, bool, int) (map[string]any, *httptest.ResponseRecorder)) {
	t.Helper()
	call("GET", "/settings/database", nil, "", "", false, 401)
	result, _ := call("GET", "/settings/database", nil, session, "", false, 200)
	if result["data"].(map[string]any)["driver"] != db.Driver {
		t.Fatal("wrong database driver")
	}
	call("PATCH", "/settings/database", map[string]int{"events": 30, "alerts": 90, "audit": 180}, session, "", false, 403)
	for _, body := range []any{map[string]int{}, map[string]int{"events": 1}, map[string]int{"events": -1, "alerts": 90, "audit": 180}, map[string]int{"events": 3651, "alerts": 90, "audit": 180}} {
		call("PATCH", "/settings/database", body, session, "", true, 400)
	}
	call("PATCH", "/settings/database", map[string]int{"events": 1, "alerts": 0, "audit": 3650}, session, "", true, 200)
	// A new App instance reads persistent settings rather than process-local state.
	app := NewApp(db, Config{})
	defer app.Shutdown()
	saved, err := app.readRetention(context.Background())
	if err != nil || saved != (retentionSettings{1, 0, 3650}) {
		t.Fatal("retention persistence", saved, err)
	}
	{
		runningApp.maintenance.mu.Lock()
		runningApp.maintenance.result.State = "running"
		runningApp.maintenance.mu.Unlock()
		call("POST", "/settings/database/reclaim", map[string]bool{"confirm": true}, session, "", true, 409)
		runningApp.maintenance.mu.Lock()
		runningApp.maintenance.result.State = "idle"
		runningApp.maintenance.mu.Unlock()
		call("POST", "/settings/database/reclaim", map[string]bool{}, session, "", true, 400)
		call("POST", "/settings/database/reclaim", map[string]bool{"confirm": true}, session, "", true, 202)
		complete := false
		for i := 0; i < 600; i++ {
			result, _ = call("GET", "/settings/database", nil, session, "", false, 200)
			state := result["data"].(map[string]any)["maintenance"].(map[string]any)["state"]
			if state == "failed" {
				t.Fatal("maintenance failed")
			}
			if state == "completed" {
				complete = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !complete {
			t.Fatal("maintenance did not finish")
		}
	}
	// Restore environment defaults for the remainder of the integration suite.
	if _, err = db.Exec(context.Background(), `DELETE FROM retention_settings`); err != nil {
		t.Fatal(err)
	}
}
