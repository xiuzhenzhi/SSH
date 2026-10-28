package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"sshguard/internal/storage"
	"strings"
	"time"
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func Bootstrap(ctx context.Context, d *storage.DB, c Config) error {
	var n int
	if e := d.Row(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); e != nil {
		return e
	}
	if n > 0 {
		return nil
	}
	if len(c.AdminPassword) < 12 || len(c.AdminPassword) > 72 {
		return fmt.Errorf("initial ADMIN_PASSWORD must be 12–72 bytes")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(c.AdminPassword), 12)
	if e != nil {
		return e
	}
	_, e = d.Exec(ctx, `INSERT INTO users(id,username,password_hash,role) VALUES(?,?,?,'admin')`, uuid.NewString(), c.AdminUsername, string(b))
	return e
}
func (a *App) login(c *gin.Context) {
	var in struct {
		Username string `json:"username" binding:"required,max=64"`
		Password string `json:"password" binding:"required,max=72"`
	}
	if !bind(c, &in) {
		return
	}
	// Fixed 15-minute account bucket is shared by replicas and independent of proxy headers.
	key := hash(strings.ToLower(in.Username) + time.Now().UTC().Truncate(15*time.Minute).Format(time.RFC3339))
	q := `INSERT INTO login_attempts(bucket_key,attempts,window_start) VALUES(?,1,?) ON CONFLICT(bucket_key) DO UPDATE SET attempts=login_attempts.attempts+1`
	if a.db.Driver == "mysql" {
		q = `INSERT INTO login_attempts(bucket_key,attempts,window_start) VALUES(?,1,?) ON DUPLICATE KEY UPDATE attempts=attempts+1`
	}
	if _, e := a.db.Exec(c, q, key, time.Now().UTC()); e != nil {
		fail(c, e)
		return
	}
	var attempts int
	if e := a.db.Row(c, `SELECT attempts FROM login_attempts WHERE bucket_key=?`, key).Scan(&attempts); e != nil {
		fail(c, e)
		return
	}
	if attempts > 10 {
		c.Header("Retry-After", "900")
		bad(c, 429, "登录尝试过多，请稍后再试")
		return
	}
	var u User
	var pw string
	e := a.db.Row(c, `SELECT id,username,role,password_hash FROM users WHERE username=?`, in.Username).Scan(&u.ID, &u.Username, &u.Role, &pw)
	if e != nil && e != sql.ErrNoRows {
		fail(c, e)
		return
	}
	if e == sql.ErrNoRows {
		pw = a.dummyHash
	}
	match := bcrypt.CompareHashAndPassword([]byte(pw), []byte(in.Password))
	if e == sql.ErrNoRows || match != nil {
		bad(c, 401, "用户名或密码错误")
		return
	}
	t := token()
	if e = a.mutate(c, u.ID, "auth.login", u.ID, func(tx *sql.Tx) error {
		var current string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT password_hash FROM users WHERE id=? FOR UPDATE`), u.ID).Scan(&current); e != nil {
			return e
		}
		if current != pw {
			return sql.ErrNoRows
		}
		_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO sessions(token_hash,user_id,expires_at) VALUES(?,?,?)`), hash(t), u.ID, time.Now().UTC().Add(12*time.Hour))
		return e
	}); e != nil {
		fail(c, e)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("sshguard_session", t, 43200, "/", "", a.cfg.SecureCookie, true)
	ok(c, u)
}
func (a *App) changePassword(c *gin.Context) {
	var in struct {
		Current string `json:"current_password" binding:"required,max=72"`
		New     string `json:"new_password" binding:"required,min=12,max=72"`
	}
	if !bind(c, &in) {
		return
	}
	if len(in.New) < 12 || len(in.New) > 72 {
		bad(c, 400, "新密码必须为 12–72 字节")
		return
	}
	u := c.MustGet("user").(User)
	var pw string
	if e := a.db.Row(c, `SELECT password_hash FROM users WHERE id=?`, u.ID).Scan(&pw); e != nil {
		fail(c, e)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(pw), []byte(in.Current)) != nil {
		bad(c, 403, "当前密码错误")
		return
	}
	b, e := bcrypt.GenerateFromPassword([]byte(in.New), 12)
	if e != nil {
		fail(c, e)
		return
	}
	e = a.mutate(c, u.ID, "auth.password.change", u.ID, func(tx *sql.Tx) error {
		var current string
		if e := tx.QueryRowContext(c, a.db.Bind(`SELECT password_hash FROM users WHERE id=? FOR UPDATE`), u.ID).Scan(&current); e != nil {
			return e
		}
		if current != pw {
			return sql.ErrNoRows
		}
		if _, e := tx.ExecContext(c, a.db.Bind(`UPDATE users SET password_hash=? WHERE id=?`), string(b), u.ID); e != nil {
			return e
		}
		_, e := tx.ExecContext(c, a.db.Bind(`DELETE FROM sessions WHERE user_id=?`), u.ID)
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("sshguard_session", "", -1, "/", "", a.cfg.SecureCookie, true)
	ok(c, gin.H{"login_required": true})
}
func (a *App) session(c *gin.Context) {
	t, e := c.Cookie("sshguard_session")
	if e != nil || len(t) != 64 {
		bad(c, 401, "请先登录")
		return
	}
	var u User
	e = a.db.Row(c, `SELECT u.id,u.username,u.role FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>?`, hash(t), time.Now().UTC()).Scan(&u.ID, &u.Username, &u.Role)
	if e == sql.ErrNoRows {
		bad(c, 401, "会话已失效")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	c.Set("user", u)
	c.Next()
}
func role(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet("user").(User)
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		bad(c, 403, "没有操作权限")
	}
}
func (a *App) logout(c *gin.Context) {
	t, _ := c.Cookie("sshguard_session")
	u := c.MustGet("user").(User)
	e := a.mutate(c, u.ID, "auth.logout", u.ID, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(c, a.db.Bind(`DELETE FROM sessions WHERE token_hash=?`), hash(t))
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("sshguard_session", "", -1, "/", "", a.cfg.SecureCookie, true)
	ok(c, gin.H{"logged_out": true})
}
func (a *App) createUser(c *gin.Context) {
	var in struct {
		Username string `json:"username" binding:"required,min=3,max=64"`
		Password string `json:"password" binding:"required,min=12,max=72"`
		Role     string `json:"role" binding:"required,oneof=admin operator viewer"`
	}
	if !bind(c, &in) {
		return
	}
	if len(in.Password) < 12 || len(in.Password) > 72 {
		bad(c, 400, "密码必须为 12–72 字节")
		return
	}
	b, e := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if e != nil {
		fail(c, e)
		return
	}
	id := uuid.NewString()
	e = a.mutate(c, c.MustGet("user").(User).ID, "user.create", id, func(tx *sql.Tx) error {
		_, e := tx.ExecContext(c, a.db.Bind(`INSERT INTO users(id,username,password_hash,role) VALUES(?,?,?,?)`), id, in.Username, string(b), in.Role)
		return e
	})
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, User{id, in.Username, in.Role})
}
