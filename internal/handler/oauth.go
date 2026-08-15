package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/FreeAlertFlow/internal/config"
	"github.com/yongheng0927/FreeAlertFlow/internal/service"
)

// OAuthHandler 提供飞书 OAuth2 登录流程（FR-5.3）：
// GET /api/auth/oauth/:provider 和 GET /api/auth/oauth/:provider/callback
type OAuthHandler struct {
	cfg     *config.Config
	oauth   *service.OAuthService // OAuth 未启用时为 nil
	states  *service.OAuthStateStore
	enabled bool
}

// NewOAuthHandler 创建 OAuthHandler，OAuth 未启用时 oauth 可为 nil
func NewOAuthHandler(cfg *config.Config, oauth *service.OAuthService, states *service.OAuthStateStore) *OAuthHandler {
	return &OAuthHandler{cfg: cfg, oauth: oauth, states: states, enabled: oauth != nil}
}

// callbackURL 基于 Root URL 推导 redirect_uri（FR-5.3/FR-6.2）
func (h *OAuthHandler) callbackURL(provider string) string {
	return strings.TrimRight(h.cfg.Server.RootURL, "/") + "/api/auth/oauth/" + provider + "/callback"
}

// Redirect 处理 GET /api/auth/oauth/:provider：携带一次性 CSRF state，
// 302 跳转到 provider 的授权页
func (h *OAuthHandler) Redirect(c *gin.Context) {
	if !h.enabled || c.Param("provider") != h.oauth.Provider().Name() {
		fail(c, http.StatusNotFound, "oauth provider not available")
		return
	}
	state, err := h.states.Issue()
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.Redirect(http.StatusFound, h.oauth.Provider().AuthURL(h.callbackURL(c.Param("provider")), state))
}

// Callback 处理 GET /api/auth/oauth/:provider/callback：校验 state，完成
// 登录，然后 302 到 SPA 的 /oauth/callback 路由，token 放在 URL fragment
// 中（fragment 不会出现在服务端日志或浏览器历史里）
func (h *OAuthHandler) Callback(c *gin.Context) {
	base := h.cfg.BasePath() + "/"
	failRedirect := func(msg string) {
		c.Redirect(http.StatusFound, base+"login?oauth_error="+url.QueryEscape(msg))
	}

	if !h.enabled || c.Param("provider") != h.oauth.Provider().Name() {
		fail(c, http.StatusNotFound, "oauth provider not available")
		return
	}
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || !h.states.Consume(state) {
		failRedirect("invalid or expired oauth state")
		return
	}
	if code == "" {
		failRedirect("missing authorization code")
		return
	}
	_, pair, err := h.oauth.LoginWithCode(c.Request.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOAuthEmailNotAllowed),
			errors.Is(err, service.ErrOAuthNotBound),
			errors.Is(err, service.ErrAccountDisabled):
			failRedirect(err.Error())
		default:
			failRedirect("oauth login failed")
		}
		return
	}
	fragment := "access_token=" + url.QueryEscape(pair.AccessToken) +
		"&refresh_token=" + url.QueryEscape(pair.RefreshToken)
	c.Redirect(http.StatusFound, base+"oauth/callback#"+fragment)
}
