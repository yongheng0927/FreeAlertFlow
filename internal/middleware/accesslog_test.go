package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// captureSlog 把默认 slog 输出重定向到 buffer，返回读取函数
func captureSlog(t *testing.T, level slog.Level) func() string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: level})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf.String
}

func TestAccessLogWritesEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	read := captureSlog(t, slog.LevelInfo)

	r := gin.New()
	r.Use(AccessLog())
	r.GET("/api/v1/alerts", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	r.ServeHTTP(w, req)

	out := read()
	for _, want := range []string{"http request", "method=GET", "path=/api/v1/alerts", "status=200", "latency=", "client_ip=10.0.0.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q: %s", want, out)
		}
	}
}

func TestAccessLogSkipsProbePaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	read := captureSlog(t, slog.LevelInfo)

	r := gin.New()
	r.Use(AccessLog())
	probe := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/healthz", probe)
	r.GET("/readyz", probe)
	r.GET("/metrics", probe)
	// 带 base path 前缀的探针同样应被跳过
	r.GET("/faf/healthz", probe)
	r.GET("/api/v1/alerts", probe)

	for _, p := range []string{"/healthz", "/readyz", "/metrics", "/faf/healthz"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
	}
	if out := read(); out != "" {
		t.Errorf("probe paths must not produce access logs, got: %s", out)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil))
	if out := read(); !strings.Contains(out, "path=/api/v1/alerts") {
		t.Errorf("business path must be logged, got: %s", out)
	}
}

func TestAccessLogServerErrorLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	read := captureSlog(t, slog.LevelInfo)

	r := gin.New()
	r.Use(AccessLog())
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if out := read(); !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "status=500") {
		t.Errorf("5xx must be logged at error level, got: %s", out)
	}
}

func TestAccessLogMasksWebhookToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	read := captureSlog(t, slog.LevelInfo)

	r := gin.New()
	r.Use(AccessLog())
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.POST("/api/v1/alerts/webhook/:token", ok)
	// 带 base path 前缀的 webhook 同样应脱敏
	r.POST("/fenghuo/api/v1/alerts/webhook/:token", ok)

	for _, p := range []string{
		"/api/v1/alerts/webhook/s3cr3t-token",
		"/fenghuo/api/v1/alerts/webhook/s3cr3t-token",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, p, nil))
	}

	out := read()
	if strings.Contains(out, "s3cr3t-token") {
		t.Errorf("webhook token must be masked in access log, got: %s", out)
	}
	if !strings.Contains(out, "/api/v1/alerts/webhook/***") {
		t.Errorf("masked webhook path must be logged, got: %s", out)
	}
}
