package api

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type retentionSettings struct {
	Events int `json:"events"`
	Alerts int `json:"alerts"`
	Audit  int `json:"audit"`
}

type maintenanceResult struct {
	State   string `json:"state"`
	Before  *int64 `json:"before,omitempty"`
	After   *int64 `json:"after,omitempty"`
	Message string `json:"message,omitempty"`
}
type databaseMaintenance struct {
	mu     sync.Mutex
	result maintenanceResult
	cancel context.CancelFunc
	done   chan struct{}
	closed bool
}

func (m *databaseMaintenance) snapshot() maintenanceResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := m.result
	if result.State == "" {
		result.State = "idle"
	}
	return result
}
func (m *databaseMaintenance) stop() {
	m.mu.Lock()
	m.closed = true
	cancel, done := m.cancel, m.done
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (a *App) readRetention(ctx context.Context) (retentionSettings, error) {
	settings := retentionSettings{a.cfg.EventRetentionDays, a.cfg.AlertRetentionDays, a.cfg.AuditRetentionDays}
	err := a.db.Row(ctx, `SELECT event_days,alert_days,audit_days FROM retention_settings WHERE id=1`).Scan(&settings.Events, &settings.Alerts, &settings.Audit)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	return settings, err
}
func (a *App) databaseSettings(c *gin.Context) {
	settings, err := a.readRetention(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	var size *int64
	if value, err := a.db.Usage(c.Request.Context()); err == nil {
		size = &value
	}
	ok(c, gin.H{"driver": a.db.Driver, "size": size, "local": a.db.Driver == "sqlite", "can_reclaim": a.db.MaintenanceAction() != "", "action": a.db.MaintenanceAction(), "retention": settings, "maintenance": a.maintenance.snapshot()})
}
func (a *App) saveDatabaseSettings(c *gin.Context) {
	var fields struct {
		Events *int `json:"events"`
		Alerts *int `json:"alerts"`
		Audit  *int `json:"audit"`
	}
	if !bind(c, &fields) {
		return
	}
	if fields.Events == nil || fields.Alerts == nil || fields.Audit == nil {
		bad(c, 400, "请提供全部三类数据的保留天数")
		return
	}
	in := retentionSettings{*fields.Events, *fields.Alerts, *fields.Audit}
	for _, days := range []int{in.Events, in.Alerts, in.Audit} {
		if days < 0 || days > 3650 {
			bad(c, 400, "保留天数须为 0–3650 的整数，0 表示永久保留")
			return
		}
	}
	err := a.mutate(c, c.MustGet("user").(User).ID, "database.retention.update", "database", func(tx *sql.Tx) error {
		var id string
		if err := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(c.Request.Context(), `DELETE FROM retention_settings WHERE id=1`); err != nil {
			return err
		}
		_, err := tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO retention_settings(id,event_days,alert_days,audit_days) VALUES(1,?,?,?)`), in.Events, in.Alerts, in.Audit)
		return err
	})
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, in)
}
func (a *App) reclaimDatabase(c *gin.Context) {
	if a.db.MaintenanceAction() == "" {
		bad(c, 400, "当前数据库不支持在线维护")
		return
	}
	var in struct {
		Confirm bool `json:"confirm"`
	}
	if !bind(c, &in) {
		return
	}
	if !in.Confirm {
		bad(c, 400, "请先确认数据库维护操作")
		return
	}
	m := &a.maintenance
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		bad(c, 503, "主控正在重启，请稍后重试")
		return
	}
	if m.result.State == "running" {
		bad(c, 409, "数据库维护正在进行，请稍后查看结果")
		return
	}
	actor := c.MustGet("user").(User).ID
	if err := a.mutate(c, actor, "database.maintenance.start", "database", func(*sql.Tx) error { return nil }); err != nil {
		fail(c, err)
		return
	}
	result := maintenanceResult{State: "running"}
	if size, err := a.db.Usage(c.Request.Context()); err == nil {
		result.Before = &size
	}
	m.result = result
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	m.cancel = cancel
	m.done = make(chan struct{})
	done := m.done
	go func(result maintenanceResult) {
		defer close(done)
		defer cancel()
		err := a.db.Reclaim(ctx)
		result.State = "completed"
		result.Message = "数据库维护完成，现有记录保持不变"
		if err != nil {
			result.State = "failed"
			result.Message = "维护未全部完成，部分表可能已优化，请查看主控日志后重试"
			log.Print("Database space reclamation failed: ", err)
		}
		if size, sizeErr := a.db.Usage(ctx); sizeErr == nil {
			result.After = &size
		}
		auditCtx, auditCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if _, auditErr := a.db.Exec(auditCtx, `INSERT INTO audit_logs(actor_id,action,target) VALUES(?,?,?)`, actor, "database.maintenance."+result.State, "database"); auditErr != nil {
			log.Print("Database maintenance audit failed")
		}
		auditCancel()
		m.mu.Lock()
		m.result = result
		m.cancel = nil
		m.mu.Unlock()
	}(result)
	c.JSON(202, gin.H{"data": result})
}
