package middleware

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog 是基于 slog 的 HTTP 访问日志中间件（NFR-3）：记录方法、路径、
// 状态码、耗时和客户端 IP。探针类高频路径（/healthz /readyz /metrics）
// 不记录，避免日志噪音；webhook 路径记录但不记 body
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if isProbePath(path) {
			return
		}
		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", time.Since(start).String(),
			"client_ip", c.ClientIP(),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.String())
		}
		if status >= 500 {
			slog.Error("http request", attrs...)
		} else {
			slog.Info("http request", attrs...)
		}
	}
}

// isProbePath 判断是否为探针/metrics 路径（含 base path 前缀的情况，按后缀匹配）
func isProbePath(path string) bool {
	for _, p := range []string{"/healthz", "/readyz", "/metrics"} {
		if path == p || strings.HasSuffix(path, p) {
			return true
		}
	}
	return false
}
