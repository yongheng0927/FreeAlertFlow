package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/service"
)

// parseIDParam 解析 :id 路径参数，要求为正整数
func parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

// pageParams 读取 ?page=&page_size= 分页参数，默认 1/20，page_size 上限 100
func pageParams(c *gin.Context) (offset, limit, page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return (page - 1) * size, size, page, size
}

// listJSON 输出统一的列表响应包络 {list, total, page, page_size}
func listJSON(c *gin.Context, list any, total int64, page, size int) {
	c.JSON(http.StatusOK, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": size,
	})
}

// parseTimeParam 解析可选的 RFC3339 时间查询参数
func parseTimeParam(c *gin.Context, key string) (*time.Time, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		fail(c, http.StatusBadRequest, key+" must be RFC3339 (e.g. 2026-08-15T02:00:00Z)")
		return nil, false
	}
	return &t, true
}

// parseInt64Query 解析可选的正整数查询参数
func parseInt64Query(c *gin.Context, key string) (*int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return nil, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v <= 0 {
		fail(c, http.StatusBadRequest, key+" must be a positive integer")
		return nil, false
	}
	return &v, true
}

// serviceError 把 service 层的哨兵错误映射为对应的 HTTP 响应
func serviceError(c *gin.Context, err error) {
	msg := err.Error()
	switch {
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, msg)
	case errors.Is(err, service.ErrValidation):
		fail(c, http.StatusBadRequest, strings.TrimPrefix(msg, "validation failed: "))
	case errors.Is(err, service.ErrDuplicateName), errors.Is(err, service.ErrReferenced):
		fail(c, http.StatusConflict, strings.TrimPrefix(msg, "resource is still referenced: "))
	case errors.Is(err, service.ErrChannelDeleted):
		fail(c, http.StatusConflict, msg)
	case errors.Is(err, service.ErrLastAdmin):
		fail(c, http.StatusConflict, msg)
	case errors.Is(err, service.ErrCannotDeleteSelf):
		fail(c, http.StatusBadRequest, msg)
	case errors.Is(err, service.ErrBadPayload):
		fail(c, http.StatusBadRequest, msg)
	default:
		fail(c, http.StatusInternalServerError, "internal error")
	}
}
