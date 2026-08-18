package handler

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/config"
)

func testConfig(rootURL string) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{HTTPAddr: ":8080", RootURL: rootURL},
	}
}

func testDist() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte(`<html><head><title>t</title><link rel="icon" href="./logo.svg" /><script src="./assets/app.js"></script></head><body>spa</body></html>`)},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log(1)")},
		"favicon.ico":   &fstest.MapFile{Data: []byte("ico")},
	}
}

func newStatic(t *testing.T, rootURL string, dist fs.FS) *StaticHandler {
	t.Helper()
	h, err := NewStaticHandler(testConfig(rootURL), dist)
	if err != nil {
		t.Fatalf("NewStaticHandler: %v", err)
	}
	return h
}

func TestStaticInjectsRuntimeConfig(t *testing.T) {
	h := newStatic(t, "https://example.com/freealertflow/", testDist())
	if !strings.Contains(string(h.indexBytes), "window.__FENGHUO_CONFIG__") {
		t.Fatal("index.html must contain the injected config")
	}
	for _, want := range []string{
		`"rootUrl":"https://example.com/freealertflow/"`,
		`"base":"/freealertflow/"`,
		`"oauthEnabled":false`,
		`"version":`,
	} {
		if !strings.Contains(string(h.indexBytes), want) {
			t.Errorf("injected config missing %s: %s", want, h.indexBytes)
		}
	}
}

func TestStaticInjectsEmptyBaseWithoutSubPath(t *testing.T) {
	h := newStatic(t, "http://localhost:8080/", testDist())
	if !strings.Contains(string(h.indexBytes), `"base":"/"`) {
		t.Errorf("base must be / : %s", h.indexBytes)
	}
}

func TestStaticRewritesRelativeAssets(t *testing.T) {
	// 子路径部署：./assets/* 必须改写为 base 下的绝对路径，否则深链接白屏
	h := newStatic(t, "https://example.com/freealertflow/", testDist())
	if !strings.Contains(string(h.indexBytes), `src="/freealertflow/assets/app.js"`) {
		t.Errorf("assets must be rewritten under base: %s", h.indexBytes)
	}
	// dist 根下的相对资源（favicon 等）同样要改写，否则子路径部署下 404
	if !strings.Contains(string(h.indexBytes), `href="/freealertflow/logo.svg"`) {
		t.Errorf("logo.svg must be rewritten under base: %s", h.indexBytes)
	}
	// 根路径部署：改写为 /assets/*
	h = newStatic(t, "http://localhost:8080/", testDist())
	if !strings.Contains(string(h.indexBytes), `src="/assets/app.js"`) {
		t.Errorf("assets must be rewritten to root: %s", h.indexBytes)
	}
	if !strings.Contains(string(h.indexBytes), `href="/logo.svg"`) {
		t.Errorf("logo.svg must be rewritten to root: %s", h.indexBytes)
	}
}

func TestServeSPAUndefBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newStatic(t, "https://example.com/freealertflow/", testDist())
	r := gin.New()
	r.NoRoute(h.ServeSPA)

	cases := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/freealertflow", 301, "/freealertflow/"},            // 裸 base 重定向加尾斜杠
		{"/freealertflow/", 200, "spa"},
		{"/freealertflow/dashboard", 200, "spa"},          // SPA 兜底
		{"/freealertflow/favicon.ico", 200, "ico"},        // 真实文件透传
		{"/freealertflow/api/v1/unknown", 404, `"error"`}, // API 未命中保持 JSON
		{"/freealertflow/metrics", 404, `"error"`},        // 探针保持 JSON
		{"/other/path", 404, `"error"`},                   // base 之外
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != tc.wantCode {
			t.Errorf("%s: code = %d, want %d", tc.path, w.Code, tc.wantCode)
			continue
		}
		if !strings.Contains(w.Body.String(), tc.wantBody) {
			t.Errorf("%s: body = %q, want containing %q", tc.path, w.Body.String(), tc.wantBody)
		}
	}
}

func TestServeSPAWithoutSubPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newStatic(t, "http://localhost:8080/", testDist())
	r := gin.New()
	r.NoRoute(h.ServeSPA)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/alerts", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "spa") {
		t.Errorf("SPA fallback: code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestServeAssetContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newStatic(t, "http://localhost:8080/", testDist())
	r := gin.New()
	r.GET("/assets/*filepath", h.ServeAsset)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "javascript") {
		t.Errorf("content-type = %q", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "console.log(1)" {
		t.Errorf("body = %q", w.Body.String())
	}
}
