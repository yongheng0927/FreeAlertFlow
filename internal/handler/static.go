package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yongheng0927/fenghuo/internal/config"
)

// StaticHandler 提供内嵌 SPA（web/dist）的静态服务，并注入 runtime 配置
// （FR-6.2）：index.html 会被插入 window.__FENGHUO_CONFIG__ 脚本标签，前端因此
// 无需针对不同 Root URL 重新构建
type StaticHandler struct {
	dist       fs.FS
	base       string // 例如 "" 或 "/freealertflow"
	indexBytes []byte // 注入了 runtime 配置的 index.html
}

// NewStaticHandler 预生成注入后的 index.html dist 是内嵌的 web/dist
// 文件系统（已 sub 到 dist 目录）
func NewStaticHandler(cfg *config.Config, dist fs.FS) (*StaticHandler, error) {
	raw, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, err
	}
	runtimeCfg := map[string]any{
		"rootUrl":      cfg.Server.RootURL,
		"base":         cfg.BasePath() + "/",
		"oauthEnabled": cfg.OAuth.Enabled,
		"version":      Version,
	}
	cfgJSON, err := json.Marshal(runtimeCfg)
	if err != nil {
		return nil, err
	}
	injection := []byte(`<script>window.__FENGHUO_CONFIG__ = ` + string(cfgJSON) + `</script>`)
	index := bytes.Replace(raw, []byte("</head>"), append(injection, []byte("</head>")...), 1)
	if !bytes.Contains(index, injection) {
		index = append(injection, raw...) // 没有 </head> 时改为插到开头
	}
	return &StaticHandler{dist: dist, base: cfg.BasePath(), indexBytes: index}, nil
}

// ServeAsset 提供内嵌 dist 中的文件（挂载在 {base}/assets/*）
func (h *StaticHandler) ServeAsset(c *gin.Context) {
	h.serveFile(c, "assets"+c.Param("filepath"))
}

// ServeSPA 是 NoRoute 兜底：dist 中存在的文件直接返回，base path 下的其余
// 路径一律返回 SPA 的 index.html base path 下的 API/探针路径仍返回 JSON 404
func (h *StaticHandler) ServeSPA(c *gin.Context) {
	p := c.Request.URL.Path
	if h.base != "" && !strings.HasPrefix(p, h.base+"/") && p != h.base {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	rel := strings.TrimPrefix(p, h.base)
	if strings.HasPrefix(rel, "/api/") || rel == "/metrics" || rel == "/healthz" || rel == "/readyz" {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		fail(c, http.StatusNotFound, "not found")
		return
	}
	if rel != "" && rel != "/" {
		name := strings.TrimPrefix(rel, "/")
		if f, err := h.dist.Open(name); err == nil {
			f.Close()
			h.serveFile(c, name)
			return
		}
	}
	h.serveIndex(c)
}

func (h *StaticHandler) serveFile(c *gin.Context, name string) {
	f, err := h.dist.Open(name)
	if err != nil {
		h.serveIndex(c)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fail(c, http.StatusInternalServerError, "internal error")
		return
	}
	http.ServeContent(c.Writer, c.Request, name, time.Time{}, bytes.NewReader(data))
}

func (h *StaticHandler) serveIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", h.indexBytes)
}
