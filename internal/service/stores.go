package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// SourceStore 抽象接入源的持久化
type SourceStore interface {
	// FindByToken 在 token 不存在时返回 (nil, nil)
	FindByToken(ctx context.Context, token string) (*model.Source, error)
	// FindByID 在接入源不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.Source, error)
	// FindByName 在名称不存在时返回 (nil, nil)
	FindByName(ctx context.Context, name string) (*model.Source, error)
	// List 返回一页接入源和总数
	List(ctx context.Context, offset, limit int) ([]model.Source, int64, error)
	Create(ctx context.Context, s *model.Source) error
	Save(ctx context.Context, s *model.Source) error
	Delete(ctx context.Context, id int64) error
	UpdateLastAlertAt(ctx context.Context, id int64, t time.Time) error
}

// AlertFilter 是告警列表接口的筛选条件（FR-4.1）
type AlertFilter struct {
	Status    string
	Severity  string
	Alertname string
	ChannelID *int64 // 通过 join deliveries 实现
	Start     *time.Time
	End       *time.Time
	Offset    int
	Limit     int
}

// AlertStore 抽象告警的持久化
type AlertStore interface {
	Create(ctx context.Context, a *model.Alert) error
	// CreateWithDedupCheck 原子完成去重判定与入库（FR-1.3）：实现必须以
	// 事务/锁保证"查窗口内上一条 + 插入"之间不插入并发告警（多副本共用
	// 数据库时同样互斥）。命中去重时新行以 disposition='deduped' 入库并
	// 返回 true；否则以 'pending' 入库等待分发 window 为 0 时关闭去重
	CreateWithDedupCheck(ctx context.Context, a *model.Alert, window time.Duration) (bool, error)
	FindByID(ctx context.Context, id int64) (*model.Alert, error)
	UpdateDisposition(ctx context.Context, id int64, disposition string) error
	// FindLatestInWindow 返回 (fingerprint, status) 相同且 received_at >= since
	// 的最近一条告警，排除 excludeID，没有时返回 (nil, nil) 去重判定
	//（FR-1.3）就靠它取窗口内的上一条告警来比对 content_hash
	FindLatestInWindow(ctx context.Context, fingerprint, status string, since time.Time, excludeID int64) (*model.Alert, error)
	// List 返回筛选后的一页告警（按 received_at 倒序）和总数
	List(ctx context.Context, f AlertFilter) ([]model.Alert, int64, error)
	CountBySource(ctx context.Context, sourceID int64) (int64, error)
	// LatestRawPayload 返回最近一条告警的 raw_payload，作为模板预览的真实
	// 数据源，没有告警时返回 nil
	LatestRawPayload(ctx context.Context) (json.RawMessage, error)
}

// RuleStore 抽象路由规则的持久化
type RuleStore interface {
	// ListEnabledBySource 返回启用的规则，按 priority 升序
	ListEnabledBySource(ctx context.Context, sourceID int64) ([]model.RoutingRule, error)
	// FindByID 在规则不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.RoutingRule, error)
	// List 返回一页规则（按 priority 升序），sourceID 为 nil 表示全部接入源
	List(ctx context.Context, sourceID *int64, offset, limit int) ([]model.RoutingRule, int64, error)
	Create(ctx context.Context, r *model.RoutingRule) error
	Save(ctx context.Context, r *model.RoutingRule) error
	Delete(ctx context.Context, id int64) error
	CountBySource(ctx context.Context, sourceID int64) (int64, error)
	CountByChannel(ctx context.Context, channelID int64) (int64, error)
}

// ChannelStore 抽象渠道的持久化
type ChannelStore interface {
	// FindByID 在渠道不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.Channel, error)
	// FindByName 在名称不存在时返回 (nil, nil)
	FindByName(ctx context.Context, name string) (*model.Channel, error)
	// List 返回一页渠道和总数
	List(ctx context.Context, offset, limit int) ([]model.Channel, int64, error)
	Create(ctx context.Context, c *model.Channel) error
	Save(ctx context.Context, c *model.Channel) error
	Delete(ctx context.Context, id int64) error
	CountByTemplate(ctx context.Context, templateID int64) (int64, error)
}

// TemplateStore 抽象模板的持久化
type TemplateStore interface {
	// FindByID 在模板不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.Template, error)
	// FindBuiltin 按渠道类型和名称查找内置模板，没有时返回 (nil, nil)
	FindBuiltin(ctx context.Context, channelType, name string) (*model.Template, error)
	// FindByName 在未找到时返回 (nil, nil)
	FindByName(ctx context.Context, channelType, name string) (*model.Template, error)
	// List 返回一页模板，channelType 为 "" 表示全部
	List(ctx context.Context, channelType string, offset, limit int) ([]model.Template, int64, error)
	Create(ctx context.Context, t *model.Template) error
	Save(ctx context.Context, t *model.Template) error
	Delete(ctx context.Context, id int64) error
}

// DeliveryFilter 是投递记录列表接口的筛选条件
type DeliveryFilter struct {
	Status    string
	ChannelID *int64
	Start     *time.Time
	End       *time.Time
	Offset    int
	Limit     int
}

// DeliveryStore 抽象投递记录的持久化
type DeliveryStore interface {
	Create(ctx context.Context, d *model.Delivery) error
	// FindByID 在投递记录不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.Delivery, error)
	// List 返回筛选后的一页（按 sent_at 倒序）和总数
	List(ctx context.Context, f DeliveryFilter) ([]model.Delivery, int64, error)
	// ListByAlertID 返回一条告警的全部投递记录（按 id 升序）
	ListByAlertID(ctx context.Context, alertID int64) ([]model.Delivery, error)
}

// OAuthIdentityStore 抽象 OAuth 身份绑定的持久化
type OAuthIdentityStore interface {
	// FindByProviderUserID 在没有绑定身份时返回 (nil, nil)
	FindByProviderUserID(ctx context.Context, provider, providerUserID string) (*model.OAuthIdentity, error)
	Create(ctx context.Context, o *model.OAuthIdentity) error
	DeleteAllForUser(ctx context.Context, userID int64) error
}
