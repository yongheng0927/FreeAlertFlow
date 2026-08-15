package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// SourceHandler 提供 /api/v1/sources 接入源管理接口
type SourceHandler struct {
	svc     *service.SourceService
	sources service.SourceStore
}

// NewSourceHandler 创建 SourceHandler
func NewSourceHandler(svc *service.SourceService, sources service.SourceStore) *SourceHandler {
	return &SourceHandler{svc: svc, sources: sources}
}

type sourceView struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Token       string     `json:"token"`
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled"`
	LastAlertAt *time.Time `json:"last_alert_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toSourceView(s *model.Source) sourceView {
	return sourceView{
		ID: s.ID, Name: s.Name, Token: s.Token, Description: s.Description,
		Enabled: s.Enabled, LastAlertAt: s.LastAlertAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

// List 处理 GET /api/v1/sources
func (h *SourceHandler) List(c *gin.Context) {
	offset, limit, page, size := pageParams(c)
	sources, total, err := h.sources.List(c.Request.Context(), offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]sourceView, 0, len(sources))
	for i := range sources {
		views = append(views, toSourceView(&sources[i]))
	}
	listJSON(c, views, total, page, size)
}

// Get 处理 GET /api/v1/sources/:id
func (h *SourceHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	src, err := h.sources.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if src == nil {
		fail(c, http.StatusNotFound, "source not found")
		return
	}
	c.JSON(http.StatusOK, toSourceView(src))
}

type sourceCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// Create 处理 POST /api/v1/sources
func (h *SourceHandler) Create(c *gin.Context) {
	var req sourceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "name is required")
		return
	}
	src, err := h.svc.Create(c.Request.Context(), req.Name, req.Description)
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toSourceView(src))
}

type sourceUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

// Update 处理 PUT /api/v1/sources/:id
func (h *SourceHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req sourceUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	src, err := h.svc.Update(c.Request.Context(), id, service.SourcePatch{
		Name: req.Name, Description: req.Description, Enabled: req.Enabled,
	})
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSourceView(src))
}

// Delete 处理 DELETE /api/v1/sources/:id
func (h *SourceHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// RotateToken 处理 POST /api/v1/sources/:id/rotate-token，旧 token 立即失效
func (h *SourceHandler) RotateToken(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	src, err := h.svc.RotateToken(c.Request.Context(), id)
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toSourceView(src))
}
