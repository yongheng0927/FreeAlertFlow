// Package handler 实现 HTTP API 层
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/service"
)

// fail 输出统一的 JSON 错误结构 {"error": "..."}
func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

// AuthHandler 提供 /api/auth/* 端点
type AuthHandler struct {
	auth    *service.AuthService
	limiter *service.LoginLimiter
}

// NewAuthHandler 创建 AuthHandler
func NewAuthHandler(auth *service.AuthService, limiter *service.LoginLimiter) *AuthHandler {
	return &AuthHandler{auth: auth, limiter: limiter}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 处理 POST /api/auth/login，带按 IP 的登录限流（NFR-1）。
// 限流器自身出错时 fail-open：记日志并放行，不因限流存储故障把正常
// 用户锁在门外
func (h *AuthHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()
	ip := c.ClientIP()
	locked, err := h.limiter.Locked(ctx, ip)
	if err != nil {
		slog.Error("login limiter check failed, allowing request", "ip", ip, "error", err)
	} else if locked {
		fail(c, http.StatusTooManyRequests, "too many failed login attempts, please try again later")
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "username and password are required")
		return
	}
	_, pair, err := h.auth.Login(ctx, req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			if err := h.limiter.Fail(ctx, ip); err != nil {
				slog.Error("login limiter fail count failed", "ip", ip, "error", err)
			}
			fail(c, http.StatusUnauthorized, err.Error())
		case errors.Is(err, service.ErrAccountDisabled):
			fail(c, http.StatusForbidden, err.Error())
		default:
			fail(c, http.StatusInternalServerError, "internal error")
		}
		return
	}
	if err := h.limiter.Reset(ctx, ip); err != nil {
		slog.Error("login limiter reset failed", "ip", ip, "error", err)
	}
	c.JSON(http.StatusOK, tokenPairResponse(pair))
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh 处理 POST /api/auth/refresh，带 token 轮换和重放检测：已轮换的
// 旧 token 一旦被重用，会吊销该用户的全部会话
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "refresh_token is required")
		return
	}
	pair, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidToken),
			errors.Is(err, service.ErrTokenReuse),
			errors.Is(err, service.ErrAccountDisabled):
			fail(c, http.StatusUnauthorized, err.Error())
		default:
			fail(c, http.StatusInternalServerError, "internal error")
		}
		return
	}
	c.JSON(http.StatusOK, tokenPairResponse(pair))
}

// Logout 处理 POST /api/auth/logout，吊销对应的 refresh token
func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "refresh_token is required")
		return
	}
	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func tokenPairResponse(pair *service.TokenPair) gin.H {
	return gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    "Bearer",
		"expires_in":    pair.ExpiresIn,
	}
}
