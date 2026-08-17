package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/yongheng0927/fenghuo/internal/config"
	"github.com/yongheng0927/fenghuo/internal/middleware"
	"github.com/yongheng0927/fenghuo/internal/model"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// Deps 汇总构建 HTTP 路由所需的全部依赖
type Deps struct {
	Config  *config.Config
	DB      *gorm.DB
	JWT     *fafjwt.Manager
	Auth    *service.AuthService
	Limiter *service.LoginLimiter

	Users   service.UserStore
	Alerts  *service.AlertService
	Sources *service.SourceService
	// 读侧接口直接使用的原始 store，不经过 service 封装
	SourceStore   service.SourceStore
	ChannelStore  service.ChannelStore
	TemplateStore service.TemplateStore
	RuleStore     service.RuleStore
	AlertStore    service.AlertStore
	DeliveryStore service.DeliveryStore
	StatsStore    service.StatsStore

	Channels  *service.ChannelService
	Templates *service.TemplateService
	Rules     *service.RuleService
	UserAdmin *service.UserAdminService
	Deliverer *service.Deliverer

	// M4：OAuth（未启用时为 nil）以及内嵌 SPA
	OAuth  *service.OAuthService
	States *service.OAuthStateStore
	Static *StaticHandler
}

// NewRouter 构建 Gin 引擎 所有路由（API、webhook、OAuth、探针、metrics、
// 静态 SPA）统一挂载在 Root URL 的路径前缀下，支持子路径部署（FR-6.2）
// /api/auth/* 和告警 webhook 保持公开，其余 /api/v1/* 需要有效 JWT
// 角色矩阵按 FR-5.4：GET 接口 viewer 起，写操作 editor 起，用户管理仅 admin
func NewRouter(d *Deps) *gin.Engine {
	r := gin.New()
	r.Use(middleware.AccessLog(), gin.Recovery())

	// 所有路由挂在 base path 下（无子路径部署时为空串）
	base := r.Group(d.Config.BasePath())

	sys := NewSystemHandler(d.Config, d.DB)
	base.GET("/healthz", sys.Healthz)
	base.GET("/readyz", sys.Readyz)
	base.GET("/metrics", gin.WrapH(promhttp.Handler()))

	authH := NewAuthHandler(d.Auth, d.Limiter)
	auth := base.Group("/api/auth")
	auth.POST("/login", authH.Login)
	auth.POST("/refresh", authH.Refresh)
	auth.POST("/logout", authH.Logout)

	oauthH := NewOAuthHandler(d.Config, d.OAuth, d.States)
	auth.GET("/oauth/:provider", oauthH.Redirect)
	auth.GET("/oauth/:provider/callback", oauthH.Callback)

	// 公开的 Alertmanager webhook 端点：凭证 token 放在 URL 路径里而非
	// header，因为 Alertmanager 的 webhook_config 配置自定义 header 很麻烦
	//（FR-1.1/FR-5.2，设计依据见 DATABASE_DESIGN §4）
	webhookH := NewWebhookHandler(d.Alerts)
	base.POST("/api/v1/alerts/webhook/:token", webhookH.Receive)

	// 公开的系统信息：登录页需要在认证前拿到 oauth_enabled 来决定是否
	// 展示 OAuth 登录入口（M4），返回字段均不敏感
	base.GET("/api/v1/system/info", sys.Info)

	userH := NewUserHandler(d.Auth, d.Users, d.Config.Admin.User)
	sourceH := NewSourceHandler(d.Sources, d.SourceStore)
	channelH := NewChannelHandler(d.Channels, d.ChannelStore)
	templateH := NewTemplateHandler(d.Templates, d.TemplateStore)
	ruleH := NewRuleHandler(d.Rules, d.RuleStore)
	alertH := NewAlertHandler(d.AlertStore, d.DeliveryStore)
	deliveryH := NewDeliveryHandler(d.DeliveryStore, d.Deliverer)
	statsH := NewStatsHandler(d.StatsStore)
	userAdminH := NewUserAdminHandler(d.Users, d.UserAdmin)

	v1 := base.Group("/api/v1", middleware.JWTAuth(d.JWT, d.Users))
	{
		// viewer 及以上角色
		v1.GET("/users/me", userH.Me)
		v1.PUT("/users/me/password", userH.ChangePassword)

		v1.GET("/sources", sourceH.List)
		v1.GET("/sources/:id", sourceH.Get)
		v1.GET("/channels", channelH.List)
		v1.GET("/channels/:id", channelH.Get)
		v1.GET("/templates", templateH.List)
		v1.GET("/templates/:id", templateH.Get)
		v1.GET("/rules", ruleH.List)
		v1.GET("/rules/:id", ruleH.Get)
		v1.GET("/alerts", alertH.List)
		v1.GET("/alerts/:id", alertH.Get)
		v1.GET("/stats/dashboard", statsH.Dashboard)
	}

	editor := v1.Group("", middleware.RequireRole(model.RoleEditor))
	{
		editor.POST("/sources", sourceH.Create)
		editor.PUT("/sources/:id", sourceH.Update)
		editor.DELETE("/sources/:id", sourceH.Delete)
		editor.POST("/sources/:id/rotate-token", sourceH.RotateToken)

		editor.POST("/channels", channelH.Create)
		editor.PUT("/channels/:id", channelH.Update)
		editor.DELETE("/channels/:id", channelH.Delete)
		editor.POST("/channels/:id/test", channelH.TestSend)

		editor.POST("/templates", templateH.Create)
		editor.PUT("/templates/:id", templateH.Update)
		editor.DELETE("/templates/:id", templateH.Delete)
		editor.POST("/templates/preview", templateH.Preview)
		editor.POST("/templates/test-send", templateH.TestSend)

		editor.POST("/rules", ruleH.Create)
		editor.PUT("/rules/:id", ruleH.Update)
		editor.DELETE("/rules/:id", ruleH.Delete)

		editor.GET("/deliveries", deliveryH.List)
		editor.POST("/deliveries/:id/resend", deliveryH.Resend)
	}

	admin := v1.Group("", middleware.RequireRole(model.RoleAdmin))
	{
		admin.GET("/users", userAdminH.List)
		admin.POST("/users", userAdminH.Create)
		admin.PUT("/users/:id/password", userAdminH.ResetPassword)
		admin.PUT("/users/:id", userAdminH.Update)
		admin.DELETE("/users/:id", userAdminH.Delete)
	}

	// SPA：内嵌静态资源 + NoRoute 回退到 index.html（FR-6.2）
	if d.Static != nil {
		base.GET("/assets/*filepath", d.Static.ServeAsset)
		r.NoRoute(d.Static.ServeSPA)
	}

	return r
}
