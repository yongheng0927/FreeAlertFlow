package handler

import (
	"testing"

	"github.com/gin-gonic/gin"
)

// TestNewRouterRegistersAllRoutes 是冒烟测试：gin 在注册阶段就会因路由冲突
// 直接 panic，因此能成功构建路由器即证明路由表自洽 注册阶段不会真正用到
// 依赖，故只传 Config
func TestNewRouterRegistersAllRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(&Deps{Config: testConfig("http://localhost:8080/")})
	want := map[string][]string{
		"GET": {
			"/healthz", "/readyz", "/metrics",
			"/api/auth/oauth/:provider", "/api/auth/oauth/:provider/callback",
			"/api/v1/users/me", "/api/v1/system/info",
			"/api/v1/sources", "/api/v1/sources/:id",
			"/api/v1/channels", "/api/v1/channels/:id",
			"/api/v1/templates", "/api/v1/templates/:id",
			"/api/v1/rules", "/api/v1/rules/:id",
			"/api/v1/alerts", "/api/v1/alerts/:id",
			"/api/v1/deliveries", "/api/v1/users",
		},
		"POST": {
			"/api/auth/login", "/api/auth/refresh", "/api/auth/logout",
			"/api/v1/alerts/webhook/:token",
			"/api/v1/sources", "/api/v1/sources/:id/rotate-token",
			"/api/v1/channels", "/api/v1/channels/:id/test",
			"/api/v1/templates", "/api/v1/templates/preview", "/api/v1/templates/test-send",
			"/api/v1/rules", "/api/v1/deliveries/:id/resend",
		},
		"PUT": {
			"/api/v1/users/me/password",
			"/api/v1/sources/:id", "/api/v1/channels/:id",
			"/api/v1/templates/:id", "/api/v1/rules/:id", "/api/v1/users/:id",
		},
		"DELETE": {
			"/api/v1/sources/:id", "/api/v1/channels/:id",
			"/api/v1/templates/:id", "/api/v1/rules/:id", "/api/v1/users/:id",
		},
	}
	registered := map[string]map[string]bool{}
	for _, ri := range r.Routes() {
		if registered[ri.Method] == nil {
			registered[ri.Method] = map[string]bool{}
		}
		registered[ri.Method][ri.Path] = true
	}
	for method, paths := range want {
		for _, p := range paths {
			if !registered[method][p] {
				t.Errorf("route %s %s not registered", method, p)
			}
		}
	}
}

// TestNewRouterBasePath 验证整套路由表都能挂载在 Root URL 的子路径下（FR-6.2）
func TestNewRouterBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(&Deps{Config: testConfig("https://example.com/freealertflow/")})
	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	for _, want := range []string{
		"GET /freealertflow/healthz",
		"GET /freealertflow/metrics",
		"POST /freealertflow/api/auth/login",
		"GET /freealertflow/api/auth/oauth/:provider/callback",
		"POST /freealertflow/api/v1/alerts/webhook/:token",
		"GET /freealertflow/api/v1/alerts",
	} {
		if !registered[want] {
			t.Errorf("route %s not registered under the base path", want)
		}
	}
}
