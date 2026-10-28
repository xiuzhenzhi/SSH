package api

import (
	"database/sql"
	"errors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"sshguard/internal/storage"
	"strconv"
	"time"
)

type App struct {
	db        *storage.DB
	cfg       Config
	dummyHash string
}

var Version = "0.2.0-alpha.1"

func New(db *storage.DB, cfg Config) *gin.Engine {
	h, _ := bcrypt.GenerateFromPassword([]byte(token()), 12)
	a := &App{db, cfg, string(h)}
	r := gin.New()
	r.Use(gin.Recovery())
	_ = r.SetTrustedProxies(nil)
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	})
	r.GET("/api/v1/health", func(c *gin.Context) {
		if e := db.PingContext(c); e != nil {
			bad(c, 503, "数据库不可用")
			return
		}
		ok(c, gin.H{"status": "ok", "database": db.Driver, "phase": 1, "version": Version})
	})
	// Cookie mutations require exact Origin; CLI clients must send Origin too.
	origin := func(c *gin.Context) {
		if c.Request.Method != "GET" && c.Request.Header.Get("Origin") != cfg.PublicURL {
			bad(c, 403, "Origin 校验失败")
			return
		}
		c.Next()
	}
	r.POST("/api/v1/auth/login", origin, a.login)
	web := r.Group("/api/v1", origin, a.session)
	web.GET("/auth/me", func(c *gin.Context) { ok(c, c.MustGet("user")) })
	web.POST("/auth/logout", a.logout)
	web.GET("/dashboard/summary", a.summary)
	web.GET("/servers", a.servers)
	web.GET("/servers/:id", a.server)
	web.POST("/servers", role("admin", "operator"), a.saveServer)
	web.PATCH("/servers/:id", role("admin", "operator"), a.saveServer)
	web.POST("/servers/:id/enrollment", role("admin"), a.enrollment)
	web.POST("/servers/:id/revoke", role("admin"), a.revoke)
	web.GET("/events", a.events)
	web.GET("/events/:id", a.event)
	web.GET("/audit-logs", role("admin"), func(c *gin.Context) {
		a.list(c, `SELECT a.id,u.username AS actor,a.action,a.target,a.created_at FROM audit_logs a LEFT JOIN users u ON a.actor_id=u.id ORDER BY a.id DESC LIMIT ? OFFSET ?`)
	})
	web.GET("/users", role("admin"), func(c *gin.Context) {
		a.list(c, `SELECT id,username,role,created_at FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?`)
	})
	web.POST("/users", role("admin"), a.createUser)
	r.POST("/api/v1/agent/enroll", a.enroll)
	agent := r.Group("/api/v1/agent", a.agentAuth)
	agent.POST("/heartbeat", a.heartbeat)
	agent.POST("/events/batch", a.batch)
	r.NoRoute(func(c *gin.Context) { bad(c, 404, "接口不存在") })
	return r
}
func ok(c *gin.Context, v any)                   { c.JSON(200, gin.H{"data": v}) }
func bad(c *gin.Context, status int, msg string) { c.AbortWithStatusJSON(status, gin.H{"error": msg}) }
func fail(c *gin.Context, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		bad(c, 404, "记录不存在或凭证已失效")
		return
	}
	log.Printf("request failed: %T", e)
	bad(c, 500, "操作失败，请检查服务端和数据库状态")
}
func bind(c *gin.Context, v any) bool {
	if e := c.ShouldBindJSON(v); e != nil {
		bad(c, 400, "请求字段或 JSON 格式无效")
		return false
	}
	return true
}
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if p < 1 {
		p = 1
	}
	if p > 100000 {
		p = 100000
	}
	if s < 1 || s > 100 {
		s = 20
	}
	return s, (p - 1) * s
}
func (a *App) list(c *gin.Context, q string, args ...any) {
	size, offset := page(c)
	args = append(args, size+1, offset)
	rows, e := a.db.List(c, q, args...)
	if e != nil {
		fail(c, e)
		return
	}
	more := len(rows) > size
	if more {
		rows = rows[:size]
	}
	ok(c, gin.H{"items": rows, "has_more": more})
}
func (a *App) mutate(c *gin.Context, actor, action, target string, fn func(*sql.Tx) error) error {
	tx, e := a.db.BeginTx(c, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(tx); e != nil {
		return e
	}
	var who any
	if actor != "" {
		who = actor
	}
	if _, e = tx.ExecContext(c, a.db.Bind(`INSERT INTO audit_logs(actor_id,action,target) VALUES(?,?,?)`), who, action, target); e != nil {
		return e
	}
	return tx.Commit()
}
func (a *App) summary(c *gin.Context) {
	since := time.Now().UTC().Add(-24 * time.Hour)
	online := time.Now().UTC().Add(-90 * time.Second)
	var total, on, rev, events, success int64
	for _, v := range []struct {
		q    string
		args []any
		dest *int64
	}{
		{`SELECT COUNT(*) FROM servers`, nil, &total}, {`SELECT COUNT(*) FROM servers WHERE revoked=false AND last_seen_at>?`, []any{online}, &on}, {`SELECT COUNT(*) FROM servers WHERE revoked=true`, nil, &rev}, {`SELECT COUNT(*) FROM ssh_events WHERE occurred_at>=?`, []any{since}, &events}, {`SELECT COUNT(*) FROM ssh_events WHERE occurred_at>=? AND outcome='success'`, []any{since}, &success},
	} {
		if e := a.db.Row(c, v.q, v.args...).Scan(v.dest); e != nil {
			fail(c, e)
			return
		}
	}
	ok(c, gin.H{"servers": total, "online": on, "offline_or_pending": total - on - rev, "revoked": rev, "events_24h": events, "success_24h": success, "failure_24h": events - success, "database": a.db.Driver})
}
