package api

import "github.com/gin-gonic/gin"

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
