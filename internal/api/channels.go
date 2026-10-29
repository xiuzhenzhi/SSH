package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"sshguard/internal/notify"
)

func (a *App) channels(c *gin.Context) {
	a.list(c, `SELECT id,name,kind,enabled,created_at,updated_at FROM notification_channels ORDER BY created_at DESC LIMIT ? OFFSET ?`)
}
func (a *App) channel(c *gin.Context) {
	var id, name, kind, sealed string
	var enabled bool
	if e := a.db.Row(c, `SELECT id,name,kind,config_ciphertext,enabled FROM notification_channels WHERE id=?`, c.Param("id")).Scan(&id, &name, &kind, &sealed, &enabled); e != nil {
		fail(c, e)
		return
	}
	b, e := notify.Open(a.cfg.EncryptionKey, id, sealed)
	if e != nil {
		bad(c, 503, "无法解密渠道配置，请检查 ENCRYPTION_KEY")
		return
	}
	var config notify.Config
	if json.Unmarshal(b, &config) != nil {
		bad(c, 500, "渠道配置损坏")
		return
	}
	configured := gin.H{"url": config.URL != "", "secret": config.Secret != "", "bot_token": config.BotToken != "", "password": config.Password != ""}
	config.URL = ""
	config.Secret = ""
	config.BotToken = ""
	config.Password = ""
	ok(c, gin.H{"id": id, "name": name, "kind": kind, "enabled": enabled, "config": config, "configured": configured})
}
func (a *App) saveChannel(c *gin.Context) {
	var in struct {
		Name           string         `json:"name" binding:"required,max=128"`
		Kind           string         `json:"kind" binding:"required,oneof=webhook wecom dingtalk feishu telegram smtp"`
		Enabled        bool           `json:"enabled"`
		Config         *notify.Config `json:"config"`
		ReplaceSecrets bool           `json:"replace_secrets"`
	}
	if !bind(c, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		bad(c, 400, "渠道名称不能为空")
		return
	}
	id := c.Param("id")
	create := id == ""
	if create {
		id = uuid.NewString()
	}
	var next notify.Config
	var previousCiphertext string
	if !create {
		var sealed, kind string
		if e := a.db.Row(c, `SELECT config_ciphertext,kind FROM notification_channels WHERE id=?`, id).Scan(&sealed, &kind); e != nil {
			fail(c, e)
			return
		}
		previousCiphertext = sealed
		if kind != in.Kind {
			bad(c, 400, "渠道类型不能修改，请新建渠道")
			return
		}
		b, e := notify.Open(a.cfg.EncryptionKey, id, sealed)
		if e != nil {
			bad(c, 503, "无法解密已有配置")
			return
		}
		if json.Unmarshal(b, &next) != nil {
			bad(c, 500, "渠道配置损坏")
			return
		}
	}
	if in.Config != nil {
		old := next
		next = *in.Config
		if !in.ReplaceSecrets {
			if next.URL == "" {
				next.URL = old.URL
			}
			if next.Secret == "" {
				next.Secret = old.Secret
			}
			if next.BotToken == "" {
				next.BotToken = old.BotToken
			}
			if next.Password == "" {
				next.Password = old.Password
			}
		}
	}
	if e := notify.Validate(in.Kind, next); e != nil {
		bad(c, 400, e.Error())
		return
	}
	b, _ := json.Marshal(next)
	sealed, e := notify.Seal(a.cfg.EncryptionKey, id, b)
	if e != nil {
		bad(c, 503, "请配置有效的 ENCRYPTION_KEY")
		return
	}
	action := "channel.update"
	if create {
		action = "channel.create"
	}
	e = a.mutate(c, c.MustGet("user").(User).ID, action, id, func(tx *sql.Tx) error {
		if create {
			var locked string
			if e := tx.QueryRowContext(c, a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&locked); e != nil {
				return e
			}
			var count int
			if e := tx.QueryRowContext(c, `SELECT COUNT(*) FROM notification_channels`).Scan(&count); e != nil {
				return e
			}
			if count >= 100 {
				return errChannelConflict
			}
			_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO notification_channels(id,name,kind,config_ciphertext,enabled) VALUES(?,?,?,?,?)`), id, in.Name, in.Kind, sealed, in.Enabled)
			return e
		}
		var current string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT config_ciphertext FROM notification_channels WHERE id=? FOR UPDATE`), id).Scan(&current); e != nil {
			return e
		}
		if current != previousCiphertext {
			return errChannelConflict
		}
		_, e := tx.ExecContext(c, a.db.Bind(`UPDATE notification_channels SET name=?,config_ciphertext=?,enabled=?,updated_at=? WHERE id=?`), in.Name, sealed, in.Enabled, time.Now().UTC(), id)
		return e
	})
	if e == errChannelConflict {
		bad(c, 409, "配置已被其他操作修改，请刷新后重试")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"id": id})
}
func (a *App) testChannel(c *gin.Context) {
	id, alertID, jobID := c.Param("id"), uuid.NewString(), uuid.NewString()
	e := a.mutate(c, c.MustGet("user").(User).ID, "channel.test", id, func(tx *sql.Tx) error {
		var enabled bool
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT enabled FROM notification_channels WHERE id=? FOR UPDATE`), id).Scan(&enabled); e != nil {
			return e
		}
		if !enabled {
			return errChannelDisabled
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO alerts(id,severity,title,message) VALUES(?,'info',?,?)`), alertID, "SSHGuard 渠道测试", "这是一条由管理员主动触发的测试通知。"); e != nil {
			return e
		}
		_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO notification_jobs(id,alert_id,channel_id,next_attempt_at) VALUES(?,?,?,?)`), jobID, alertID, id, time.Now().UTC())
		return e
	})
	if e == errChannelDisabled {
		bad(c, 409, "请先启用渠道再测试")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"job_id": jobID, "alert_id": alertID, "status": "pending"})
}

var errChannelDisabled = errors.New("channel disabled or task not retryable")
var errChannelConflict = errors.New("channel changed concurrently")

func (a *App) deliveries(c *gin.Context) {
	q := `SELECT j.id,j.alert_id,j.channel_id,a.title AS alert_title,ch.name AS channel_name,ch.kind,j.status,j.attempts,j.next_attempt_at,j.last_error,j.sent_at,j.created_at FROM notification_jobs j JOIN notification_channels ch ON ch.id=j.channel_id JOIN alerts a ON a.id=j.alert_id WHERE 1=1`
	args := []any{}
	if v := c.Query("status"); v != "" {
		q += " AND j.status=?"
		args = append(args, v)
	}
	if v := c.Query("alert_id"); v != "" {
		q += " AND j.alert_id=?"
		args = append(args, v)
	}
	a.list(c, q+` ORDER BY j.created_at DESC LIMIT ? OFFSET ?`, args...)
}
func (a *App) retryDelivery(c *gin.Context) {
	id := c.Param("id")
	e := a.mutate(c, c.MustGet("user").(User).ID, "notification.retry", id, func(tx *sql.Tx) error {
		var status string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT status FROM notification_jobs WHERE id=? FOR UPDATE`), id).Scan(&status); e != nil {
			return e
		}
		if status != "dead" {
			return errChannelDisabled
		}
		_, e := tx.ExecContext(c, a.db.Bind(`UPDATE notification_jobs SET status='pending',attempts=0,next_attempt_at=?,locked_until=NULL,lease_token='',last_error='' WHERE id=?`), time.Now().UTC(), id)
		return e
	})
	if e == errChannelDisabled {
		bad(c, 409, "只有失败终止的任务可以手动重试")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"queued": true})
}
