package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/middleware"
	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// UserHandler 提供 /api/v1/users/me 端点
type UserHandler struct {
	auth  *service.AuthService
	users service.UserStore
}

// NewUserHandler 创建 UserHandler
func NewUserHandler(auth *service.AuthService, users service.UserStore) *UserHandler {
	return &UserHandler{auth: auth, users: users}
}

type userResponse struct {
	ID          int64      `json:"id"`
	Username    string     `json:"username"`
	Name        string     `json:"name"`
	Email       string     `json:"email"`
	AvatarURL   string     `json:"avatar_url"`
	Role        string     `json:"role"`
	Enabled     bool       `json:"enabled"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toUserResponse(u *model.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Username:    u.Username,
		Name:        u.Name,
		Email:       u.Email,
		AvatarURL:   u.AvatarURL,
		Role:        u.Role,
		Enabled:     u.Enabled,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

// Me 处理 GET /api/v1/users/me
func (h *UserHandler) Me(c *gin.Context) {
	userID := c.GetInt64(middleware.CtxUserID)
	user, err := h.users.FindByID(c.Request.Context(), userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		fail(c, http.StatusUnauthorized, "account disabled or deleted")
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword 处理 PUT /api/v1/users/me/password 成功后吊销该用户的
// 全部 refresh token，所有已登录端都需要重新登录
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "old_password and new_password are required")
		return
	}
	if len(req.NewPassword) < 8 {
		fail(c, http.StatusBadRequest, "new password must be at least 8 characters")
		return
	}
	userID := c.GetInt64(middleware.CtxUserID)
	err := h.auth.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			fail(c, http.StatusBadRequest, "old password is incorrect")
		case errors.Is(err, service.ErrNoLocalPassword):
			fail(c, http.StatusBadRequest, err.Error())
		default:
			fail(c, http.StatusInternalServerError, "internal error")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated, please log in again"})
}
