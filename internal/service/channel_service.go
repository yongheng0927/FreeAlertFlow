package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// MaskCredential 对凭证脱敏用于 API 返回：只显示最后 4 个字符
// （FR-2.1，NFR-1） URL 会保留路径前缀，例如
// "https://open.feishu.cn/open-apis/bot/v2/hook/****ab12"
func MaskCredential(s string) string {
	if s == "" {
		return ""
	}
	tail := s
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	if i := strings.LastIndex(s, "/"); i > 0 && i < len(s)-5 {
		return s[:i+1] + "****" + tail
	}
	return "****" + tail
}

// ChannelService 实现渠道管理：凭证明文存储、脱敏读取、带引用检查的删除和
// 测试发送（FR-2.1/2.5）
type ChannelService struct {
	channels  ChannelStore
	rules     RuleStore
	templates TemplateStore
	sender    Sender
}

// NewChannelService 创建 ChannelService
func NewChannelService(channels ChannelStore, rules RuleStore, templates TemplateStore,
	sender Sender) *ChannelService {
	return &ChannelService{channels: channels, rules: rules, templates: templates, sender: sender}
}

// ChannelInput 承载渠道的创建/更新字段
type ChannelInput struct {
	Name       string
	Type       string // feishu / dingtalk / wecom；空 = feishu（兼容旧调用方）
	WebhookURL string // 更新时为空 = 保持原值
	Keyword    string
	TemplateID *int64 // 更新时 nil = 保持原值，解绑用 ClearTemplate
	AtAll      bool
	Enabled    bool
}

// channelWebhookHosts 是各渠道机器人 webhook 的合法主机名
var channelWebhookHosts = map[string]string{
	model.ChannelTypeFeishu:   "open.feishu.cn",
	model.ChannelTypeDingTalk: "oapi.dingtalk.com",
	model.ChannelTypeWeCom:    "qyapi.weixin.qq.com",
}

func validChannelType(t string) bool {
	_, ok := channelWebhookHosts[t]
	return ok
}

// ChannelPatch 承载可选的更新字段
type ChannelPatch struct {
	Name          *string
	WebhookURL    *string // nil 或空 = 保持原值
	Secret        *string // nil = 保持原值，"" = 清除，有值 = 替换
	Keyword       *string
	TemplateID    *int64 // nil = 保持原值
	ClearTemplate bool   // true = 解绑模板
	AtAll         *bool
	Enabled       *bool
}

func validateWebhookURL(raw, chType string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return validationErr("webhook_url is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return validationErr("webhook_url must be http(s)")
	}
	if host, ok := channelWebhookHosts[chType]; ok &&
		!strings.EqualFold(u.Hostname(), host) {
		return validationErr("webhook_url host must be %s for %s channels", host, chType)
	}
	return nil
}

// Create 创建渠道，明文存储 webhook URL 和签名密钥（FR-2.2）
func (s *ChannelService) Create(ctx context.Context, in ChannelInput, secret string) (*model.Channel, error) {
	if in.Name == "" {
		return nil, validationErr("name is required")
	}
	chType := in.Type
	if chType == "" {
		chType = model.ChannelTypeFeishu
	}
	if !validChannelType(chType) {
		return nil, validationErr("type must be feishu, dingtalk or wecom")
	}
	if in.WebhookURL == "" {
		return nil, validationErr("webhook_url is required")
	}
	if err := validateWebhookURL(in.WebhookURL, chType); err != nil {
		return nil, err
	}
	existing, err := s.channels.FindByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
	}
	if err := s.checkTemplate(ctx, chType, in.TemplateID); err != nil {
		return nil, err
	}
	ch := &model.Channel{
		Name:       in.Name,
		Type:       chType,
		WebhookURL: in.WebhookURL,
		Keyword:    in.Keyword,
		TemplateID: in.TemplateID,
		AtAll:      in.AtAll,
		Enabled:    in.Enabled,
	}
	// 企业微信机器人无加签机制，secret 仅对 feishu/dingtalk 有效
	if secret != "" && chType != model.ChannelTypeWeCom {
		ch.Secret = secret
	}
	if err := s.channels.Create(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// Update 应用补丁，凭证除非显式替换否则保持原值（FR-2.1：编辑时不重新
// 提交则保留原值）
func (s *ChannelService) Update(ctx context.Context, id int64, p ChannelPatch) (*model.Channel, error) {
	ch, err := s.channels.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, fmt.Errorf("%w: channel %d", ErrNotFound, id)
	}
	if p.Name != nil {
		if *p.Name == "" {
			return nil, validationErr("name must not be empty")
		}
		if *p.Name != ch.Name {
			existing, err := s.channels.FindByName(ctx, *p.Name)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				return nil, fmt.Errorf("%w: %q", ErrDuplicateName, *p.Name)
			}
		}
		ch.Name = *p.Name
	}
	if p.WebhookURL != nil && *p.WebhookURL != "" {
		if err := validateWebhookURL(*p.WebhookURL, ch.Type); err != nil {
			return nil, err
		}
		ch.WebhookURL = *p.WebhookURL
	}
	if p.Secret != nil && ch.Type != model.ChannelTypeWeCom { // 企微无加签，忽略
		ch.Secret = *p.Secret // 空串即显式清除
	}
	if p.Keyword != nil {
		ch.Keyword = *p.Keyword
	}
	if p.ClearTemplate {
		ch.TemplateID = nil
	} else if p.TemplateID != nil {
		if err := s.checkTemplate(ctx, ch.Type, p.TemplateID); err != nil {
			return nil, err
		}
		ch.TemplateID = p.TemplateID
	}
	if p.AtAll != nil {
		ch.AtAll = *p.AtAll
	}
	if p.Enabled != nil {
		ch.Enabled = *p.Enabled
	}
	if err := s.channels.Save(ctx, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

// Delete 删除渠道，仍有路由规则引用时拒绝（DB 设计 §11）
func (s *ChannelService) Delete(ctx context.Context, id int64) error {
	ch, err := s.channels.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if ch == nil {
		return fmt.Errorf("%w: channel %d", ErrNotFound, id)
	}
	n, err := s.rules.CountByChannel(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return referencedErr("channel is referenced by %d routing rules", n)
	}
	return s.channels.Delete(ctx, id)
}

// TestSendResult 是测试发送的结果（FR-2.5）
type TestSendResult struct {
	Success    bool   `json:"success"`
	HTTPStatus int    `json:"http_status"`
	Code       int    `json:"code"`
	Msg        string `json:"msg"`
	DurationMS int64  `json:"duration_ms"`
}

// TestSend 立即通过渠道发送一条测试消息，验证 URL + 签名是否正确
// （FR-2.5） 有意不写 deliveries 记录：测试发送没有关联告警，而
// deliveries.alert_id 在逻辑上指向 alerts
func (s *ChannelService) TestSend(ctx context.Context, id int64) (*TestSendResult, error) {
	ch, err := s.channels.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, fmt.Errorf("%w: channel %d", ErrNotFound, id)
	}
	text := "烽火台测试消息：渠道配置验证成功"
	if ch.Keyword != "" && !strings.Contains(text, ch.Keyword) {
		text = ch.Keyword + " " + text
	}
	// 测试消息按渠道类型直接构造样例消息体；AtAll 由 Sender 层注入
	payload, err := testSamplePayload(ch.Type, text)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	res := s.sender.Send(ctx, ch, payload)
	return &TestSendResult{
		Success:    res.Success(),
		HTTPStatus: res.HTTPStatus,
		Code:       res.Code,
		Msg:        res.Message(),
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// testSamplePayload 为测试发送构造指定渠道类型的样例消息体
func testSamplePayload(chType, text string) ([]byte, error) {
	var payload map[string]any
	switch chType {
	case model.ChannelTypeFeishu:
		payload = map[string]any{
			"msg_type": "text",
			"content":  map[string]any{"text": text},
		}
	case model.ChannelTypeDingTalk:
		payload = map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]any{"title": "烽火台测试消息", "text": text},
		}
	case model.ChannelTypeWeCom:
		payload = map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]any{"content": text},
		}
	default:
		return nil, validationErr("unsupported channel type %q", chType)
	}
	return json.Marshal(payload) // map[string]any 序列化不会失败
}

func (s *ChannelService) checkTemplate(ctx context.Context, chType string, templateID *int64) error {
	if templateID == nil {
		return nil
	}
	tmpl, err := s.templates.FindByID(ctx, *templateID)
	if err != nil {
		return err
	}
	if tmpl == nil {
		return validationErr("template %d does not exist", *templateID)
	}
	// 模板按渠道类型归属，只能绑定与渠道类型一致的模板
	if tmpl.ChannelType != chType {
		return validationErr("template %d is for %s channels, not %s", *templateID, tmpl.ChannelType, chType)
	}
	return nil
}

// ChannelView 是渠道的 API 表示，凭证已脱敏（FR-2.1：只显示最后 4 个
// 字符）
type ChannelView struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	WebhookURL string    `json:"webhook_url"` // 已脱敏
	HasSecret  bool      `json:"has_secret"`
	Secret     string    `json:"secret"` // 已脱敏，未设置时为 ""
	Keyword    string    `json:"keyword"`
	TemplateID *int64    `json:"template_id"`
	AtAll      bool      `json:"at_all"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// View 构建渠道的脱敏 API 视图
func (s *ChannelService) View(ch *model.Channel) (*ChannelView, error) {
	v := &ChannelView{
		ID: ch.ID, Name: ch.Name, Type: ch.Type, Keyword: ch.Keyword,
		TemplateID: ch.TemplateID, AtAll: ch.AtAll, Enabled: ch.Enabled,
		CreatedAt: ch.CreatedAt, UpdatedAt: ch.UpdatedAt,
	}
	v.WebhookURL = MaskCredential(ch.WebhookURL)
	if ch.Secret != "" {
		v.HasSecret = true
		v.Secret = MaskCredential(ch.Secret)
	}
	return v, nil
}
