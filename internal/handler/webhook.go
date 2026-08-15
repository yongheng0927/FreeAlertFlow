package handler

import (
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/FreeAlertFlow/internal/service"
)

// maxWebhookBody 限制 webhook 请求体大小，1 MiB 对 Alertmanager 分组消息
// 足够宽裕
const maxWebhookBody = 1 << 20

// WebhookHandler 提供公开的 Alertmanager webhook 接收端点
type WebhookHandler struct {
	alerts *service.AlertService
}

// NewWebhookHandler 创建 WebhookHandler
func NewWebhookHandler(alerts *service.AlertService) *WebhookHandler {
	return &WebhookHandler{alerts: alerts}
}

// Receive 处理 POST /api/v1/alerts/webhook/:token（FR-1.1） 接入是同步的
// （解析 + 入库），路由分发和渠道投递异步执行，因此响应很快（NFR-2）
func (h *WebhookHandler) Receive(c *gin.Context) {
	token := c.Param("token")
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBody))
	if err != nil {
		fail(c, http.StatusBadRequest, "read request body failed")
		return
	}
	// webhook 负载是 JSON，按定义必然是 UTF-8；PostgreSQL 遇到非法 UTF-8
	// 会报错，最终在响应里表现为 500，因此在入口处直接拒绝
	if !utf8.Valid(body) {
		fail(c, http.StatusBadRequest, "request body must be valid UTF-8")
		return
	}
	_, n, err := h.alerts.Ingest(c.Request.Context(), token, body)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSourceNotFound):
			fail(c, http.StatusNotFound, err.Error())
		case errors.Is(err, service.ErrSourceDisabled):
			fail(c, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrBadPayload):
			fail(c, http.StatusBadRequest, err.Error())
		default:
			fail(c, http.StatusInternalServerError, "internal error")
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "alerts": n})
}
