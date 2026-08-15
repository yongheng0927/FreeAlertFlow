package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/FreeAlertFlow/internal/service"
)

// ChannelHandler 提供 /api/v1/channels 渠道管理接口
type ChannelHandler struct {
	svc      *service.ChannelService
	channels service.ChannelStore
}

// NewChannelHandler 创建 ChannelHandler
func NewChannelHandler(svc *service.ChannelService, channels service.ChannelStore) *ChannelHandler {
	return &ChannelHandler{svc: svc, channels: channels}
}

// List 处理 GET /api/v1/channels
func (h *ChannelHandler) List(c *gin.Context) {
	offset, limit, page, size := pageParams(c)
	channels, total, err := h.channels.List(c.Request.Context(), offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]*service.ChannelView, 0, len(channels))
	for i := range channels {
		v, err := h.svc.View(&channels[i])
		if err != nil {
			fail(c, http.StatusInternalServerError, "internal error")
			return
		}
		views = append(views, v)
	}
	listJSON(c, views, total, page, size)
}

// Get 处理 GET /api/v1/channels/:id
func (h *ChannelHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	ch, err := h.channels.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if ch == nil {
		fail(c, http.StatusNotFound, "channel not found")
		return
	}
	v, err := h.svc.View(ch)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, v)
}

type channelCreateRequest struct {
	Name       string `json:"name" binding:"required"`
	WebhookURL string `json:"webhook_url" binding:"required"`
	Secret     string `json:"secret"`
	Keyword    string `json:"keyword"`
	TemplateID *int64 `json:"template_id"`
	AtAll      bool   `json:"at_all"`
	Enabled    *bool  `json:"enabled"`
}

// Create 处理 POST /api/v1/channels
func (h *ChannelHandler) Create(c *gin.Context) {
	var req channelCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "name and webhook_url are required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	ch, err := h.svc.Create(c.Request.Context(), service.ChannelInput{
		Name:       req.Name,
		WebhookURL: req.WebhookURL,
		Keyword:    req.Keyword,
		TemplateID: req.TemplateID,
		AtAll:      req.AtAll,
		Enabled:    enabled,
	}, req.Secret)
	if err != nil {
		serviceError(c, err)
		return
	}
	h.respondView(c, http.StatusCreated, ch.ID)
}

type channelUpdateRequest struct {
	Name       *string `json:"name"`
	WebhookURL *string `json:"webhook_url"` // 缺省/空 = 保持原值
	Secret     *string `json:"secret"`      // 缺省 = 保持原值，"" = 清除
	Keyword    *string `json:"keyword"`
	TemplateID *int64  `json:"template_id"` // 缺省 = 保持原值，0 = 解绑
	AtAll      *bool   `json:"at_all"`
	Enabled    *bool   `json:"enabled"`
}

// Update 处理 PUT /api/v1/channels/:id
func (h *ChannelHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req channelUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	patch := service.ChannelPatch{
		Name: req.Name, WebhookURL: req.WebhookURL, Secret: req.Secret,
		Keyword: req.Keyword, AtAll: req.AtAll, Enabled: req.Enabled,
	}
	if req.TemplateID != nil {
		if *req.TemplateID == 0 {
			patch.ClearTemplate = true
		} else {
			patch.TemplateID = req.TemplateID
		}
	}
	ch, err := h.svc.Update(c.Request.Context(), id, patch)
	if err != nil {
		serviceError(c, err)
		return
	}
	h.respondView(c, http.StatusOK, ch.ID)
}

// Delete 处理 DELETE /api/v1/channels/:id
func (h *ChannelHandler) Delete(c *gin.Context) {
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

// TestSend 处理 POST /api/v1/channels/:id/test（FR-2.5）
func (h *ChannelHandler) TestSend(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	res, err := h.svc.TestSend(c.Request.Context(), id)
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// respondView 重新加载渠道并输出脱敏后的视图
func (h *ChannelHandler) respondView(c *gin.Context, status int, id int64) {
	ch, err := h.channels.FindByID(c.Request.Context(), id)
	if err != nil || ch == nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	v, err := h.svc.View(ch)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(status, v)
}
