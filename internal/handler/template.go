package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/service"
)

// TemplateHandler 提供 /api/v1/templates 模板管理接口
type TemplateHandler struct {
	svc       *service.TemplateService
	templates service.TemplateStore
}

// NewTemplateHandler 创建 TemplateHandler
func NewTemplateHandler(svc *service.TemplateService, templates service.TemplateStore) *TemplateHandler {
	return &TemplateHandler{svc: svc, templates: templates}
}

type templateView struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ChannelType string    `json:"channel_type"`
	Content     string    `json:"content"`
	IsBuiltin   bool      `json:"is_builtin"`
	Remark      string    `json:"remark"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toTemplateView(t *model.Template) templateView {
	return templateView{
		ID: t.ID, Name: t.Name, ChannelType: t.ChannelType, Content: t.Content,
		IsBuiltin: t.IsBuiltin, Remark: t.Remark,
		CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

// List 处理 GET /api/v1/templates?channel_type=
func (h *TemplateHandler) List(c *gin.Context) {
	offset, limit, page, size := pageParams(c)
	templates, total, err := h.templates.List(c.Request.Context(), c.Query("channel_type"), offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]templateView, 0, len(templates))
	for i := range templates {
		views = append(views, toTemplateView(&templates[i]))
	}
	listJSON(c, views, total, page, size)
}

// Get 处理 GET /api/v1/templates/:id
func (h *TemplateHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	tmpl, err := h.templates.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if tmpl == nil {
		fail(c, http.StatusNotFound, "template not found")
		return
	}
	c.JSON(http.StatusOK, toTemplateView(tmpl))
}

type templateSaveRequest struct {
	Name        string `json:"name" binding:"required"`
	ChannelType string `json:"channel_type" binding:"required"`
	Content     string `json:"content" binding:"required"`
	Remark      string `json:"remark"`
}

// Create 处理 POST /api/v1/templates
func (h *TemplateHandler) Create(c *gin.Context) {
	var req templateSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "name, channel_type and content are required")
		return
	}
	tmpl, err := h.svc.Create(c.Request.Context(), service.TemplateInput{
		Name: req.Name, ChannelType: req.ChannelType, Content: req.Content, Remark: req.Remark,
	})
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toTemplateView(tmpl))
}

// Update 处理 PUT /api/v1/templates/:id
func (h *TemplateHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req templateSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "name, channel_type and content are required")
		return
	}
	tmpl, err := h.svc.Update(c.Request.Context(), id, service.TemplateInput{
		Name: req.Name, ChannelType: req.ChannelType, Content: req.Content, Remark: req.Remark,
	})
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTemplateView(tmpl))
}

// Delete 处理 DELETE /api/v1/templates/:id
func (h *TemplateHandler) Delete(c *gin.Context) {
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

type templatePreviewRequest struct {
	Content     string          `json:"content"`
	ChannelType string          `json:"channel_type"` // content 直传时缺省 feishu
	TemplateID  *int64          `json:"template_id"`
	Alert       json.RawMessage `json:"alert"` // 可选的 Alertmanager v4 负载
}

// Preview 处理 POST /api/v1/templates/preview（FR-2.3 在线预览）
func (h *TemplateHandler) Preview(c *gin.Context) {
	var req templatePreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	content := req.Content
	channelType := req.ChannelType
	if content == "" && req.TemplateID != nil {
		tmpl, err := h.templates.FindByID(c.Request.Context(), *req.TemplateID)
		if err != nil {
			fail(c, http.StatusInternalServerError, "internal error")
			return
		}
		if tmpl == nil {
			fail(c, http.StatusNotFound, "template not found")
			return
		}
		content = tmpl.Content
		// 用 template_id 时以模板自己的渠道类型校验
		channelType = tmpl.ChannelType
	}
	if content == "" {
		fail(c, http.StatusBadRequest, "content or template_id is required")
		return
	}
	if channelType == "" {
		channelType = model.ChannelTypeFeishu
	}
	rendered, err := h.svc.Preview(c.Request.Context(), content, channelType, req.Alert)
	if err != nil {
		serviceError(c, err)
		return
	}
	// 与前端约定的契约：rendered 是渲染出的渠道消息体 JSON 字符串
	c.JSON(http.StatusOK, gin.H{"rendered": rendered})
}
