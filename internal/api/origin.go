package api

import (
	"sshguard/internal/originurl"

	"github.com/gin-gonic/gin"
)

// Direct HTTP uses the actual request host, never untrusted proxy headers.
// HTTPS reverse proxies must configure PUBLIC_URL explicitly.
func (a *App) browserOrigin(c *gin.Context) string {
	if a.cfg.PublicURL != "" {
		return a.cfg.PublicURL
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// Use the address the administrator actually opened in direct mode. Forwarded
// headers never choose where an Agent downloads code or sends credentials.
func (a *App) agentOrigin(c *gin.Context) (string, bool) {
	origin := a.browserOrigin(c)
	u, err := originurl.Parse(origin)
	if err != nil {
		bad(c, 409, "主控访问地址无效，请使用服务器 IP 和端口重新打开控制台")
		return "", false
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !a.cfg.AllowInsecureHTTP) {
		bad(c, 409, "请启用 HTTPS，或在主控配置中显式启用 ALLOW_INSECURE_HTTP")
		return "", false
	}
	return origin, true
}
