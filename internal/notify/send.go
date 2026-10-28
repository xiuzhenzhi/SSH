package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Config struct {
	URL      string   `json:"url,omitempty"`
	Secret   string   `json:"secret,omitempty"`
	BotToken string   `json:"bot_token,omitempty"`
	ChatID   string   `json:"chat_id,omitempty"`
	Host     string   `json:"host,omitempty"`
	Port     int      `json:"port,omitempty"`
	TLSMode  string   `json:"tls_mode,omitempty"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	From     string   `json:"from,omitempty"`
	To       []string `json:"to,omitempty"`
}
type Message struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Text  string    `json:"text"`
	At    time.Time `json:"at"`
}
type Sender struct {
	AllowedHosts []string
	RootCAs      *x509.CertPool
}

func Validate(kind string, c Config) error {
	if len(c.Secret) > 512 || len(c.Password) > 512 || len(c.Username) > 128 {
		return errors.New("channel field too long")
	}
	switch kind {
	case "webhook", "wecom", "dingtalk", "feishu":
		u, e := url.Parse(c.URL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(c.URL) > 2048 {
			return errors.New("webhook must be an HTTPS URL without userinfo or fragment")
		}
	case "telegram":
		if len(c.BotToken) < 10 || len(c.BotToken) > 256 || strings.ContainsAny(c.BotToken, "/?#\r\n ") || len(c.ChatID) < 1 || len(c.ChatID) > 128 {
			return errors.New("invalid Telegram token or chat ID")
		}
	case "smtp":
		if c.Host == "" || len(c.Host) > 253 || strings.ContainsAny(c.Host, "/@\\\r\n ") || c.Port < 1 || c.Port > 65535 || (c.TLSMode != "starttls" && c.TLSMode != "tls") {
			return errors.New("SMTP requires a host, port and verified TLS mode (starttls or tls)")
		}
		if len(c.To) < 1 || len(c.To) > 20 {
			return errors.New("SMTP requires 1–20 recipients")
		}
		for _, v := range append([]string{c.From}, c.To...) {
			if strings.ContainsAny(v, "\r\n") || len(v) > 254 {
				return errors.New("invalid email address")
			}
			if _, e := mail.ParseAddress(v); e != nil {
				return errors.New("invalid email address")
			}
		}
	default:
		return errors.New("unsupported notification kind")
	}
	return nil
}

func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96"} {
		_, n, _ := net.ParseCIDR(cidr)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

// Resolve and validate each connection, then dial the checked address directly.
// This prevents redirects, proxy bypass and DNS rebinding to local services.
func (s Sender) dial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, errors.New("invalid destination")
	}
	allowed := false
	for _, h := range s.AllowedHosts {
		if strings.EqualFold(strings.TrimSpace(h), host) {
			allowed = true
		}
	}
	ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
	if e != nil {
		return nil, errors.New("destination DNS lookup failed")
	}
	for _, ip := range ips {
		if !allowed && !publicIP(ip.IP) {
			return nil, errors.New("private destination blocked; explicitly allow its hostname in NOTIFICATION_ALLOWED_HOSTS")
		}
	}
	for _, ip := range ips {
		c, e := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if e == nil {
			return c, nil
		}
	}
	return nil, errors.New("destination connection failed")
}
func (s Sender) Send(ctx context.Context, kind string, c Config, m Message) error {
	if e := Validate(kind, c); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if kind == "smtp" {
		return s.sendSMTP(ctx, c, m)
	}
	endpoint, payload := c.URL, any(m)
	content := m.Title + "\n" + m.Text
	if len(content) > 1800 {
		end := 1800
		for end > 0 && !utf8.RuneStart(content[end]) {
			end--
		}
		content = content[:end]
	}
	text := content + "\nAlert ID: " + m.ID
	now := time.Now()
	switch kind {
	case "wecom":
		payload = map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
	case "dingtalk":
		payload = map[string]any{"msgtype": "text", "text": map[string]string{"content": text}}
		if c.Secret != "" {
			stamp := strconv.FormatInt(now.UnixMilli(), 10)
			u, _ := url.Parse(endpoint)
			q := u.Query()
			q.Set("timestamp", stamp)
			q.Set("sign", signature(c.Secret, stamp+"\n"+c.Secret))
			u.RawQuery = q.Encode()
			endpoint = u.String()
		}
	case "feishu":
		p := map[string]any{"msg_type": "text", "content": map[string]string{"text": text}}
		if c.Secret != "" {
			stamp := strconv.FormatInt(now.Unix(), 10)
			p["timestamp"] = stamp
			p["sign"] = signature(stamp+"\n"+c.Secret, "")
		}
		payload = p
	case "telegram":
		endpoint = "https://api.telegram.org/bot" + c.BotToken + "/sendMessage"
		payload = map[string]any{"chat_id": c.ChatID, "text": text, "disable_web_page_preview": true}
	}
	body, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if e != nil {
		return errors.New("invalid notification request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", m.ID)
	if kind == "webhook" && c.Secret != "" {
		mac := hmac.New(sha256.New, []byte(c.Secret))
		mac.Write(body)
		req.Header.Set("X-SSHGuard-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	tr := &http.Transport{DialContext: s.dial, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: s.RootCAs}, TLSHandshakeTimeout: 10 * time.Second, DisableKeepAlives: true, Proxy: nil}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return errors.New("HTTPS delivery failed (check DNS, TLS and destination policy)")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 65537))
	if e != nil || len(b) > 65536 {
		return errors.New("invalid notification response")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("notification HTTP status %d", res.StatusCode)
	}
	if kind != "webhook" {
		var response struct {
			OK         bool `json:"ok"`
			Code       *int `json:"code"`
			ErrCode    *int `json:"errcode"`
			StatusCode *int `json:"StatusCode"`
		}
		if json.Unmarshal(b, &response) != nil {
			return errors.New("notification provider returned invalid JSON")
		}
		if kind == "telegram" && !response.OK {
			return errors.New("Telegram rejected notification")
		}
		if kind != "telegram" && response.Code == nil && response.ErrCode == nil && response.StatusCode == nil {
			return errors.New("notification provider omitted acknowledgement")
		}
		for _, code := range []*int{response.Code, response.ErrCode, response.StatusCode} {
			if code != nil && *code != 0 {
				return fmt.Errorf("notification provider rejected message (code %d)", *code)
			}
		}
	}
	return nil
}
func signature(key, message string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func (s Sender) sendSMTP(ctx context.Context, c Config, m Message) error {
	conn, e := s.dial(ctx, "tcp", net.JoinHostPort(c.Host, strconv.Itoa(c.Port)))
	if e != nil {
		return e
	}
	raw := conn
	defer raw.Close()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { raw.Close() })
	defer stop()
	tlsConfig := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12, RootCAs: s.RootCAs}
	if c.TLSMode == "tls" {
		secure := tls.Client(conn, tlsConfig)
		if e = secure.HandshakeContext(ctx); e != nil {
			return errors.New("SMTP TLS handshake failed")
		}
		conn = secure
	}
	client, e := smtp.NewClient(conn, c.Host)
	if e != nil {
		return errors.New("SMTP greeting failed")
	}
	defer client.Close()
	if c.TLSMode == "starttls" {
		if e = client.StartTLS(tlsConfig); e != nil {
			return errors.New("SMTP STARTTLS required but failed")
		}
	}
	if c.Username != "" {
		if e = client.Auth(smtp.PlainAuth("", c.Username, c.Password, c.Host)); e != nil {
			return errors.New("SMTP authentication failed")
		}
	}
	from, _ := mail.ParseAddress(c.From)
	if e = client.Mail(from.Address); e != nil {
		return errors.New("SMTP sender rejected")
	}
	for _, recipient := range c.To {
		to, _ := mail.ParseAddress(recipient)
		if e = client.Rcpt(to.Address); e != nil {
			return errors.New("SMTP recipient rejected")
		}
	}
	w, e := client.Data()
	if e != nil {
		return errors.New("SMTP DATA rejected")
	}
	message := "From: " + from.String() + "\r\nTo: " + strings.Join(c.To, ", ") + "\r\nSubject: SSHGuard security alert\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMessage-ID: <" + m.ID + "@sshguard.local>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(m.Title + "\n" + m.Text + "\nAlert ID: " + m.ID))
	for len(encoded) > 76 {
		message += encoded[:76] + "\r\n"
		encoded = encoded[76:]
	}
	message += encoded + "\r\n"
	if _, e = io.WriteString(w, message); e != nil {
		return errors.New("SMTP message write failed")
	}
	if e = w.Close(); e != nil {
		return errors.New("SMTP message rejected")
	}
	// DATA acceptance is success even if the peer closes before acknowledging QUIT.
	_ = client.Quit()
	return nil
}
