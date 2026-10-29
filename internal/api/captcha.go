package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sshguard/internal/notify"
	"sshguard/internal/originurl"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type CaptchaConfig struct {
	Provider  string   `json:"provider" binding:"required,oneof=none turnstile recaptcha geetest"`
	SiteKey   string   `json:"site_key" binding:"max=256"`
	Secret    string   `json:"secret" binding:"max=512"`
	Hostnames []string `json:"hostnames" binding:"max=10,dive,max=253"`
}
type CaptchaAnswer struct {
	Token  string `json:"token" binding:"max=8192"`
	Lot    string `json:"lot_number" binding:"max=128"`
	Output string `json:"captcha_output" binding:"max=8192"`
	Pass   string `json:"pass_token" binding:"max=2048"`
	Time   string `json:"gen_time" binding:"max=32"`
}

var errCaptchaInvalid = errors.New("invalid human verification")
var errCaptchaUnavailable = errors.New("human verification unavailable")
var captchaHTTP = &http.Client{Timeout: 8 * time.Second, Transport: func() *http.Transport { t := http.DefaultTransport.(*http.Transport).Clone(); t.Proxy = nil; return t }(), CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func canonicalCaptchaHost(host string) (string, bool) {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.String(), true
	}
	u, e := originurl.Parse("https://" + host)
	if e != nil || u.Hostname() != host || u.Port() != "" {
		return "", false
	}
	return host, true
}
func (a *App) readCaptcha(ctx context.Context) (CaptchaConfig, error) {
	var raw string
	e := a.db.Row(ctx, `SELECT config_ciphertext FROM login_protection WHERE id=1`).Scan(&raw)
	if e == sql.ErrNoRows {
		return CaptchaConfig{Provider: "none"}, nil
	}
	if e != nil {
		return CaptchaConfig{}, e
	}
	plain, e := notify.Open(a.cfg.EncryptionKey, "login-protection", raw)
	if e != nil {
		return CaptchaConfig{}, e
	}
	var cfg CaptchaConfig
	e = json.Unmarshal(plain, &cfg)
	return cfg, e
}
func publicCaptcha(cfg CaptchaConfig) gin.H {
	return gin.H{"provider": cfg.Provider, "site_key": cfg.SiteKey}
}
func (a *App) loginProtection(c *gin.Context) {
	cfg, e := a.readCaptcha(c.Request.Context())
	if e != nil {
		bad(c, 503, "暂时无法加载登录防护")
		return
	}
	ok(c, publicCaptcha(cfg))
}
func (a *App) captchaSettings(c *gin.Context) {
	cfg, e := a.readCaptcha(c.Request.Context())
	if e != nil {
		fail(c, e)
		return
	}
	value := publicCaptcha(cfg)
	value["hostnames"] = cfg.Hostnames
	value["secret_configured"] = cfg.Secret != ""
	ok(c, value)
}
func (a *App) saveCaptchaSettings(c *gin.Context) {
	var in struct {
		CaptchaConfig
		Password string `json:"current_password" binding:"required,max=72"`
	}
	if !bind(c, &in) {
		return
	}
	if len(in.Password) > 72 {
		bad(c, 403, "当前密码错误")
		return
	}
	user := c.MustGet("user").(User)
	var passwordHash string
	if e := a.db.Row(c.Request.Context(), `SELECT password_hash FROM users WHERE id=?`, user.ID).Scan(&passwordHash); e != nil {
		fail(c, e)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(in.Password)) != nil {
		bad(c, 403, "当前密码错误")
		return
	}
	if in.Provider != "none" {
		if strings.TrimSpace(in.SiteKey) == "" || len(in.Hostnames) == 0 {
			bad(c, 400, "请填写站点标识与允许的主机名")
			return
		}
		for i, host := range in.Hostnames {
			value, valid := canonicalCaptchaHost(host)
			if !valid {
				bad(c, 400, "允许的主机名不能包含端口、路径或通配符")
				return
			}
			in.Hostnames[i] = value
		}
	} else {
		in.SiteKey = ""
		in.Secret = ""
		in.Hostnames = nil
	}
	e := a.mutate(c, user.ID, "auth.protection.update", "login-protection", func(tx *sql.Tx) error {
		var locked string
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1 FOR UPDATE`)).Scan(&locked); e != nil {
			return e
		}
		if e := tx.QueryRowContext(c.Request.Context(), a.db.Bind(`SELECT password_hash FROM users WHERE id=? FOR UPDATE`), user.ID).Scan(&locked); e != nil {
			return e
		}
		if locked != passwordHash {
			return sql.ErrNoRows
		}
		var oldRaw string
		e := tx.QueryRowContext(c.Request.Context(), `SELECT config_ciphertext FROM login_protection WHERE id=1`).Scan(&oldRaw)
		exists := e == nil
		if e != nil && e != sql.ErrNoRows {
			return e
		}
		if in.Provider != "none" && in.Secret == "" && exists {
			plain, e := notify.Open(a.cfg.EncryptionKey, "login-protection", oldRaw)
			if e != nil {
				return e
			}
			var old CaptchaConfig
			if e = json.Unmarshal(plain, &old); e != nil {
				return e
			}
			if old.Provider == in.Provider && old.SiteKey == in.SiteKey {
				in.Secret = old.Secret
			}
		}
		if in.Provider != "none" && in.Secret == "" {
			return errCaptchaInvalid
		}
		raw, e := json.Marshal(in.CaptchaConfig)
		if e != nil {
			return e
		}
		sealed, e := notify.Seal(a.cfg.EncryptionKey, "login-protection", raw)
		if e != nil {
			return e
		}
		if exists {
			_, e = tx.ExecContext(c.Request.Context(), a.db.Bind(`UPDATE login_protection SET config_ciphertext=? WHERE id=1`), sealed)
		} else {
			_, e = tx.ExecContext(c.Request.Context(), a.db.Bind(`INSERT INTO login_protection(id,config_ciphertext) VALUES(1,?)`), sealed)
		}
		return e
	})
	if errors.Is(e, errCaptchaInvalid) {
		bad(c, 400, "首次配置或切换站点时必须填写密钥")
		return
	}
	if e != nil {
		fail(c, e)
		return
	}
	ok(c, gin.H{"saved": true})
}

func verifyCaptcha(ctx context.Context, client *http.Client, cfg CaptchaConfig, answer CaptchaAnswer, ip string, now time.Time) error {
	if cfg.Provider == "none" {
		return nil
	}
	if cfg.Secret == "" || cfg.SiteKey == "" || len(cfg.Hostnames) == 0 {
		return errCaptchaUnavailable
	}
	form := url.Values{}
	endpoint := ""
	switch cfg.Provider {
	case "turnstile":
		endpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	case "recaptcha":
		endpoint = "https://www.google.com/recaptcha/api/siteverify"
	case "geetest":
		endpoint = "https://gcaptcha4.geetest.com/validate"
		if answer.Lot == "" || answer.Output == "" || answer.Pass == "" || answer.Time == "" {
			return errCaptchaInvalid
		}
		seconds, e := strconv.ParseInt(answer.Time, 10, 64)
		if e != nil || time.Unix(seconds, 0).Before(now.Add(-5*time.Minute)) || time.Unix(seconds, 0).After(now.Add(30*time.Second)) {
			return errCaptchaInvalid
		}
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write([]byte(answer.Lot))
		form = url.Values{"captcha_id": {cfg.SiteKey}, "lot_number": {answer.Lot}, "captcha_output": {answer.Output}, "pass_token": {answer.Pass}, "gen_time": {answer.Time}, "sign_token": {hex.EncodeToString(mac.Sum(nil))}}
	default:
		return errCaptchaUnavailable
	}
	if cfg.Provider != "geetest" {
		if answer.Token == "" {
			return errCaptchaInvalid
		}
		form = url.Values{"secret": {cfg.Secret}, "response": {answer.Token}, "remoteip": {ip}}
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return errCaptchaUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, e := client.Do(req)
	if e != nil {
		return errCaptchaUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return errCaptchaUnavailable
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, 16385))
	if e != nil || len(raw) > 16384 {
		return errCaptchaUnavailable
	}
	var result struct {
		Success  bool      `json:"success"`
		Hostname string    `json:"hostname"`
		Action   string    `json:"action"`
		At       time.Time `json:"challenge_ts"`
		Errors   []string  `json:"error-codes"`
		Result   string    `json:"result"`
		Args     struct {
			Lot     string `json:"lot_number"`
			Referer string `json:"referer"`
		} `json:"captcha_args"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return errCaptchaUnavailable
	}
	host := result.Hostname
	if cfg.Provider == "geetest" {
		if result.Result != "success" || result.Args.Lot != answer.Lot {
			return errCaptchaInvalid
		}
		u, e := url.Parse(result.Args.Referer)
		if e != nil || (u.Scheme != "https" && u.Scheme != "http") {
			return errCaptchaInvalid
		}
		host = u.Hostname()
	} else {
		if !result.Success {
			for _, code := range result.Errors {
				if code == "internal-error" || code == "invalid-input-secret" || code == "missing-input-secret" {
					return errCaptchaUnavailable
				}
			}
			return errCaptchaInvalid
		}
		age := 5 * time.Minute
		if cfg.Provider == "recaptcha" {
			age = 2 * time.Minute
		}
		if result.At.Before(now.Add(-age)) || result.At.After(now.Add(30*time.Second)) {
			return errCaptchaInvalid
		}
		if cfg.Provider == "turnstile" && result.Action != "login" {
			return errCaptchaInvalid
		}
	}
	host, valid := canonicalCaptchaHost(host)
	if !valid {
		return errCaptchaInvalid
	}
	for _, allowed := range cfg.Hostnames {
		if host == allowed {
			return nil
		}
	}
	return errCaptchaInvalid
}

func (a *App) checkCaptcha(c *gin.Context, answer CaptchaAnswer, ip string) error {
	cfg, e := a.readCaptcha(c.Request.Context())
	if e != nil {
		return errCaptchaUnavailable
	}
	if cfg.Provider == "none" {
		return nil
	}
	if e = verifyCaptcha(c.Request.Context(), captchaHTTP, cfg, answer, ip, time.Now().UTC()); e != nil {
		return e
	}
	identity := answer.Token
	if cfg.Provider == "geetest" {
		identity = answer.Lot + "|" + answer.Pass
	}
	q := `INSERT INTO captcha_used(token_hash,expires_at) VALUES(?,?) ON CONFLICT(token_hash) DO NOTHING`
	if a.db.Driver == "mysql" {
		// INSERT IGNORE reports zero for duplicates even with clientFoundRows enabled.
		q = `INSERT IGNORE INTO captcha_used(token_hash,expires_at) VALUES(?,?)`
	}
	result, e := a.db.Exec(c.Request.Context(), q, hash(cfg.Provider+"|"+identity), time.Now().UTC().Add(10*time.Minute))
	if e != nil {
		return errCaptchaUnavailable
	}
	n, e := result.RowsAffected()
	if e != nil {
		return errCaptchaUnavailable
	}
	if n == 0 {
		return errCaptchaInvalid
	}
	return nil
}
