package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/service"
)

// SetupHandler 提供 /api/v1/setup 首次启动引导端点（FR-5.1，公开访问）
type SetupHandler struct {
	auth    *service.AuthService
	limiter setupLimiter
}

// setupLimiter 抽象 setup 复用的登录限流（由 *service.LoginLimiter 实现），
// 便于 handler 层测试
type setupLimiter interface {
	Locked(ctx context.Context, ip string) (bool, error)
	Fail(ctx context.Context, ip string) error
	Reset(ctx context.Context, ip string) error
}

// NewSetupHandler 创建 SetupHandler
func NewSetupHandler(auth *service.AuthService, limiter setupLimiter) *SetupHandler {
	return &SetupHandler{auth: auth, limiter: limiter}
}

// Status 处理 GET /api/v1/setup/status：报告初始管理员是否已设置
func (h *SetupHandler) Status(c *gin.Context) {
	ok, err := h.auth.Initialized(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"initialized": ok})
}

type setupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Setup 处理 POST /api/v1/setup：创建初始管理员并直接签发 token 对。
// 复用登录限流的语义（NFR-1）：先查 IP 锁定，失败计数、成功清零；
// 限流器自身出错时 fail-open，记日志放行
func (h *SetupHandler) Setup(c *gin.Context) {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	locked, err := h.limiter.Locked(ctx, ip)
	if err != nil {
		slog.Error("login limiter check failed, allowing request", "ip", ip, "error", err)
	} else if locked {
		fail(c, http.StatusTooManyRequests, "too many failed login attempts, please try again later")
		return
	}
	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "username and password are required")
		return
	}
	pair, err := h.auth.Setup(ctx, req.Username, req.Password)
	if err != nil {
		if err := h.limiter.Fail(ctx, ip); err != nil {
			slog.Error("login limiter fail count failed", "ip", ip, "error", err)
		}
		switch {
		case errors.Is(err, service.ErrSetupCompleted):
			fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrValidation):
			fail(c, http.StatusBadRequest, err.Error())
		default:
			fail(c, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if err := h.limiter.Reset(ctx, ip); err != nil {
		slog.Error("login limiter reset failed", "ip", ip, "error", err)
	}
	c.JSON(http.StatusCreated, tokenPairResponse(pair))
}
