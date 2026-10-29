package api

import (
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// Only public bundle paths are exposed. os.Root confines symlinks and traversal
// to the web directory; never serve the database or the release directory.
func (a *App) serveWeb(c *gin.Context) {
	requested := c.Request.URL.Path
	if a.cfg.WebDir == "" || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) ||
		strings.HasPrefix(requested, "/api") || strings.Contains(requested, "\\") {
		bad(c, 404, "接口不存在")
		return
	}
	for _, segment := range strings.Split(requested, "/") {
		if strings.HasPrefix(segment, ".") {
			bad(c, 404, "文件不存在")
			return
		}
	}
	name := strings.TrimPrefix(path.Clean(requested), "/")
	if name == "assets" || name == "downloads" {
		bad(c, 404, "文件不存在")
		return
	}
	switch {
	case name == "." || name == "" || name == "index.html":
		name = "index.html"
	case strings.HasPrefix(name, "assets/"), strings.HasPrefix(name, "downloads/"):
	default:
		// The console uses one entry page; unknown file-like paths stay 404.
		if path.Ext(name) != "" {
			bad(c, 404, "文件不存在")
			return
		}
		name = "index.html"
	}
	root, err := os.OpenRoot(a.cfg.WebDir)
	if err != nil {
		bad(c, 404, "网页资源不可用")
		return
	}
	defer root.Close()
	file, err := root.Open(name)
	if err != nil {
		bad(c, 404, "文件不存在")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		bad(c, 404, "文件不存在")
		return
	}
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' https://challenges.cloudflare.com/turnstile/ https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/ https://static.geetest.com/ https://static.geevisit.com/ https://gcaptcha4.geetest.com/ https://gcaptcha4.geevisit.com/; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https://static.geetest.com https://static.geevisit.com https://gcaptcha4.geetest.com; connect-src 'self' https://challenges.cloudflare.com https://www.google.com/recaptcha/ https://gcaptcha4.geetest.com https://gcaptcha4.geevisit.com; frame-src 'self' https://challenges.cloudflare.com https://www.google.com/recaptcha/ https://recaptcha.google.com/recaptcha/ https://static.geetest.com https://static.geevisit.com; worker-src 'self' blob:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	if strings.HasPrefix(name, "downloads/") {
		c.Header("Content-Type", "application/octet-stream")
	}
	// NoRoute begins with 404; ServeContent must write the successful status.
	c.Status(http.StatusOK)
	http.ServeContent(c.Writer, c.Request, path.Base(name), info.ModTime(), file)
}
