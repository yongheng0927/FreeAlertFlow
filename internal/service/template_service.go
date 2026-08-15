package service

import (
	"context"
	"fmt"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
	"github.com/yongheng0927/FreeAlertFlow/internal/pkg/render"
)

// ErrBuiltinReadOnly 表示试图修改或删除内置模板
var ErrBuiltinReadOnly = fmt.Errorf("%w: builtin templates are read-only (copy them instead)", ErrValidation)

// TemplateService 实现模板管理：保存时渲染校验、内置模板保护和预览接口
// （FR-2.3）
type TemplateService struct {
	templates TemplateStore
	channels  ChannelStore
	alerts    AlertStore
	engine    *render.Engine
	rootURL   string
}

// NewTemplateService 创建 TemplateService
func NewTemplateService(templates TemplateStore, channels ChannelStore, alerts AlertStore,
	engine *render.Engine, rootURL string) *TemplateService {
	return &TemplateService{templates: templates, channels: channels, alerts: alerts, engine: engine, rootURL: rootURL}
}

// TemplateInput 承载模板的创建/更新字段
type TemplateInput struct {
	Name        string
	ChannelType string
	Content     string
	Remark      string
}

// validateInput 校验必填字段以及内容的可渲染性
func (s *TemplateService) validateInput(in TemplateInput) error {
	if in.Name == "" {
		return validationErr("name is required")
	}
	if in.ChannelType != "feishu" {
		return validationErr("channel_type must be feishu in V1")
	}
	if in.Content == "" {
		return validationErr("content is required")
	}
	// 渲染结果必须是合法的 IM 消息体（FR-2.3）
	if _, err := s.engine.Render(in.Content, render.SampleContext()); err != nil {
		return validationErr("template does not render a valid message: %v", err)
	}
	return nil
}

// Create 校验并保存自定义模板
func (s *TemplateService) Create(ctx context.Context, in TemplateInput) (*model.Template, error) {
	if err := s.validateInput(in); err != nil {
		return nil, err
	}
	existing, err := s.templates.FindByName(ctx, in.ChannelType, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
	}
	tmpl := &model.Template{
		Name:        in.Name,
		ChannelType: in.ChannelType,
		Content:     in.Content,
		Remark:      in.Remark,
	}
	if err := s.templates.Create(ctx, tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Update 修改自定义模板，内置模板只读
func (s *TemplateService) Update(ctx context.Context, id int64, in TemplateInput) (*model.Template, error) {
	tmpl, err := s.templates.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tmpl == nil {
		return nil, fmt.Errorf("%w: template %d", ErrNotFound, id)
	}
	if tmpl.IsBuiltin {
		return nil, ErrBuiltinReadOnly
	}
	if err := s.validateInput(in); err != nil {
		return nil, err
	}
	if in.Name != tmpl.Name || in.ChannelType != tmpl.ChannelType {
		existing, err := s.templates.FindByName(ctx, in.ChannelType, in.Name)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != id {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
		}
	}
	tmpl.Name = in.Name
	tmpl.ChannelType = in.ChannelType
	tmpl.Content = in.Content
	tmpl.Remark = in.Remark
	if err := s.templates.Save(ctx, tmpl); err != nil {
		return nil, err
	}
	return tmpl, nil
}

// Delete 删除自定义模板，仍有渠道引用时拒绝（DB 设计 §11）
func (s *TemplateService) Delete(ctx context.Context, id int64) error {
	tmpl, err := s.templates.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if tmpl == nil {
		return fmt.Errorf("%w: template %d", ErrNotFound, id)
	}
	if tmpl.IsBuiltin {
		return ErrBuiltinReadOnly
	}
	n, err := s.channels.CountByTemplate(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return referencedErr("template is bound to %d channels", n)
	}
	return s.templates.Delete(ctx, id)
}

// samplePayload 是预览/校验的内置兜底负载
const samplePayload = `{
  "version": "4",
  "groupKey": "{}:{alertname=\"HighCPU\"}",
  "truncatedAlerts": 0,
  "status": "firing",
  "receiver": "freealertflow",
  "groupLabels": {"alertname": "HighCPU"},
  "commonLabels": {"alertname": "HighCPU", "severity": "critical", "instance": "10.0.0.1:9100", "namespace": "prod"},
  "commonAnnotations": {"summary": "CPU usage above 90% for 5m", "description": "Node 10.0.0.1 CPU is on fire"},
  "externalURL": "http://alertmanager:9093",
  "alerts": [
    {
      "status": "firing",
      "labels": {"alertname": "HighCPU", "severity": "critical", "instance": "10.0.0.1:9100", "namespace": "prod"},
      "annotations": {"summary": "CPU usage above 90% for 5m", "description": "Node 10.0.0.1 CPU is on fire"},
      "startsAt": "2026-08-15T02:00:00.000Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://prometheus:9090/graph",
      "fingerprint": "0123456789abcdef"
    }
  ]
}`

// Preview 用告警负载渲染模板（content 或 template_id）：优先用调用方提供
// 的 JSON，否则用最近一条入库告警，最后用内置样例（FR-2.3 在线预览）
func (s *TemplateService) Preview(ctx context.Context, content string, alertJSON []byte) (string, error) {
	if content == "" {
		return "", validationErr("content is required")
	}
	payload := alertJSON
	if len(payload) == 0 {
		latest, err := s.alerts.LatestRawPayload(ctx)
		if err != nil {
			return "", err
		}
		payload = latest
	}
	if len(payload) == 0 {
		payload = []byte(samplePayload)
	}
	msg, err := ParseAMWebhook(payload)
	if err != nil {
		return "", err
	}
	rctx := buildRenderContext(msg, &model.Alert{}, "模板预览", s.rootURL)
	return s.engine.Render(content, rctx)
}
