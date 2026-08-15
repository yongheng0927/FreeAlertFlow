package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/FreeAlertFlow/internal/middleware"
	"github.com/yongheng0927/FreeAlertFlow/internal/service"
)

// UserAdminHandler 提供 /api/v1/users 用户管理接口（仅 admin，FR-5.4）
type UserAdminHandler struct {
	users service.UserStore
	svc   *service.UserAdminService
}

// NewUserAdminHandler 创建 UserAdminHandler
func NewUserAdminHandler(users service.UserStore, svc *service.UserAdminService) *UserAdminHandler {
	return &UserAdminHandler{users: users, svc: svc}
}

// List 处理 GET /api/v1/users
func (h *UserAdminHandler) List(c *gin.Context) {
	offset, limit, page, size := pageParams(c)
	users, total, err := h.users.List(c.Request.Context(), offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]userResponse, 0, len(users))
	for i := range users {
		views = append(views, toUserResponse(&users[i]))
	}
	listJSON(c, views, total, page, size)
}

type userUpdateRequest struct {
	Role    *string `json:"role"`
	Enabled *bool   `json:"enabled"`
}

// Update 处理 PUT /api/v1/users/:id（改角色、启用/禁用）
func (h *UserAdminHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req userUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Role == nil && req.Enabled == nil {
		fail(c, http.StatusBadRequest, "nothing to update: provide role and/or enabled")
		return
	}
	user, err := h.svc.Update(c.Request.Context(), id, req.Role, req.Enabled)
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}

// Delete 处理 DELETE /api/v1/users/:id
func (h *UserAdminHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	actorID := c.GetInt64(middleware.CtxUserID)
	if err := h.svc.Delete(c.Request.Context(), actorID, id); err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
