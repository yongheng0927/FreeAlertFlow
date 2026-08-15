package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/crypto"
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

// ChannelService 实现渠道管理：凭证加密、脱敏读取、带引用检查的删除和
// 测试发送（FR-2.1/2.5）
type ChannelService struct {
	channels  ChannelStore
	rules     RuleStore
	templates TemplateStore
	cipher    *crypto.Cipher
	sender    Sender
}

// NewChannelService 创建 ChannelService
func NewChannelService(channels ChannelStore, rules RuleStore, templates TemplateStore,
	cipher *crypto.Cipher, sender Sender) *ChannelService {
	return &ChannelService{channels: channels, rules: rules, templates: templates, cipher: cipher, sender: sender}
}

// ChannelInput 承载渠道的创建/更新字段
type ChannelInput struct {
	Name       string
	WebhookURL string // 更新时为空 = 保持原值
	Keyword    string
	TemplateID *int64 // 更新时 nil = 保持原值，解绑用 ClearTemplate
	AtAll      bool
	Enabled    bool
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

func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return validationErr("webhook_url is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return validationErr("webhook_url must be http(s)")
	}
	return nil
}

// Create 创建渠道，加密存储 webhook URL 和签名密钥（FR-2.2）
func (s *ChannelService) Create(ctx context.Context, in ChannelInput, secret string) (*model.Channel, error) {
	if in.Name == "" {
		return nil, validationErr("name is required")
	}
	if in.WebhookURL == "" {
		return nil, validationErr("webhook_url is required")
	}
	if err := validateWebhookURL(in.WebhookURL); err != nil {
		return nil, err
	}
	existing, err := s.channels.FindByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
	}
	if err := s.checkTemplate(ctx, in.TemplateID); err != nil {
		return nil, err
	}
	encURL, err := s.cipher.Encrypt([]byte(in.WebhookURL))
	if err != nil {
		return nil, err
	}
	ch := &model.Channel{
		Name:                in.Name,
		Type:                "feishu",
		WebhookURLEncrypted: encURL,
		Keyword:             in.Keyword,
		TemplateID:          in.TemplateID,
		AtAll:               in.AtAll,
		Enabled:             in.Enabled,
	}
	if secret != "" {
		enc, err := s.cipher.Encrypt([]byte(secret))
		if err != nil {
			return nil, err
		}
		ch.SecretEncrypted = &enc
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
		if err := validateWebhookURL(*p.WebhookURL); err != nil {
			return nil, err
		}
		enc, err := s.cipher.Encrypt([]byte(*p.WebhookURL))
		if err != nil {
			return nil, err
		}
		ch.WebhookURLEncrypted = enc
	}
	if p.Secret != nil {
		if *p.Secret == "" {
			ch.SecretEncrypted = nil // 显式清除
		} else {
			enc, err := s.cipher.Encrypt([]byte(*p.Secret))
			if err != nil {
				return nil, err
			}
			ch.SecretEncrypted = &enc
		}
	}
	if p.Keyword != nil {
		ch.Keyword = *p.Keyword
	}
	if p.ClearTemplate {
		ch.TemplateID = nil
	} else if p.TemplateID != nil {
		if err := s.checkTemplate(ctx, p.TemplateID); err != nil {
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
	payload := fmt.Sprintf(`{"msg_type":"text","content":{"text":%q}}`, text)
	start := time.Now()
	res := s.sender.Send(ctx, ch, []byte(payload))
	return &TestSendResult{
		Success:    res.Success(),
		HTTPStatus: res.HTTPStatus,
		Code:       res.Code,
		Msg:        res.Message(),
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

func (s *ChannelService) checkTemplate(ctx context.Context, templateID *int64) error {
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
	url, err := s.cipher.Decrypt(ch.WebhookURLEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt webhook url of channel %d: %w", ch.ID, err)
	}
	v.WebhookURL = MaskCredential(string(url))
	if ch.SecretEncrypted != nil && len(*ch.SecretEncrypted) > 0 {
		secret, err := s.cipher.Decrypt(*ch.SecretEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt secret of channel %d: %w", ch.ID, err)
		}
		v.HasSecret = true
		v.Secret = MaskCredential(string(secret))
	}
	return v, nil
}
