package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
	"github.com/yongheng0927/FreeAlertFlow/internal/service"
)

// RuleHandler 提供 /api/v1/rules 路由规则管理接口
type RuleHandler struct {
	svc   *service.RuleService
	rules service.RuleStore
}

// NewRuleHandler 创建 RuleHandler
func NewRuleHandler(svc *service.RuleService, rules service.RuleStore) *RuleHandler {
	return &RuleHandler{svc: svc, rules: rules}
}

type ruleView struct {
	ID               int64           `json:"id"`
	SourceID         int64           `json:"source_id"`
	Name             string          `json:"name"`
	Priority         int             `json:"priority"`
	MatchLabels      json.RawMessage `json:"match_labels"`
	ChannelID        int64           `json:"channel_id"`
	ContinueMatching bool            `json:"continue_matching"`
	Enabled          bool            `json:"enabled"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func toRuleView(r *model.RoutingRule) ruleView {
	return ruleView{
		ID: r.ID, SourceID: r.SourceID, Name: r.Name, Priority: r.Priority,
		MatchLabels: r.MatchLabels, ChannelID: r.ChannelID,
		ContinueMatching: r.ContinueMatching, Enabled: r.Enabled,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// List 处理 GET /api/v1/rules?source_id=
func (h *RuleHandler) List(c *gin.Context) {
	sourceID, ok := parseInt64Query(c, "source_id")
	if !ok {
		return
	}
	offset, limit, page, size := pageParams(c)
	rules, total, err := h.rules.List(c.Request.Context(), sourceID, offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	views := make([]ruleView, 0, len(rules))
	for i := range rules {
		views = append(views, toRuleView(&rules[i]))
	}
	listJSON(c, views, total, page, size)
}

// Get 处理 GET /api/v1/rules/:id
func (h *RuleHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	r, err := h.rules.FindByID(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	if r == nil {
		fail(c, http.StatusNotFound, "rule not found")
		return
	}
	c.JSON(http.StatusOK, toRuleView(r))
}

type ruleSaveRequest struct {
	SourceID         int64           `json:"source_id" binding:"required"`
	Name             string          `json:"name"`
	Priority         int             `json:"priority"`
	MatchLabels      json.RawMessage `json:"match_labels" binding:"required"`
	ChannelID        int64           `json:"channel_id" binding:"required"`
	ContinueMatching bool            `json:"continue_matching"`
	Enabled          *bool           `json:"enabled"`
}

func (r ruleSaveRequest) toInput() service.RuleInput {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return service.RuleInput{
		SourceID: r.SourceID, Name: r.Name, Priority: r.Priority,
		MatchLabels: r.MatchLabels, ChannelID: r.ChannelID,
		ContinueMatching: r.ContinueMatching, Enabled: enabled,
	}
}

// Create 处理 POST /api/v1/rules
func (h *RuleHandler) Create(c *gin.Context) {
	var req ruleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "source_id, match_labels and channel_id are required")
		return
	}
	r, err := h.svc.Create(c.Request.Context(), req.toInput())
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toRuleView(r))
}

// Update 处理 PUT /api/v1/rules/:id
func (h *RuleHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req ruleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "source_id, match_labels and channel_id are required")
		return
	}
	r, err := h.svc.Update(c.Request.Context(), id, req.toInput())
	if err != nil {
		serviceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toRuleView(r))
}

// Delete 处理 DELETE /api/v1/rules/:id
func (h *RuleHandler) Delete(c *gin.Context) {
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
