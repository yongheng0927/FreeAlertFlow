package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// GormSourceStore 用 GORM/PostgreSQL 实现 SourceStore
type GormSourceStore struct{ db *gorm.DB }

func NewGormSourceStore(db *gorm.DB) *GormSourceStore { return &GormSourceStore{db: db} }

func (s *GormSourceStore) FindByToken(ctx context.Context, token string) (*model.Source, error) {
	var src model.Source
	err := s.db.WithContext(ctx).Where("token = ?", token).Take(&src).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *GormSourceStore) FindByID(ctx context.Context, id int64) (*model.Source, error) {
	var src model.Source
	err := s.db.WithContext(ctx).Take(&src, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *GormSourceStore) FindByName(ctx context.Context, name string) (*model.Source, error) {
	var src model.Source
	err := s.db.WithContext(ctx).Where("name = ?", name).Take(&src).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}

func (s *GormSourceStore) List(ctx context.Context, offset, limit int) ([]model.Source, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.Source{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var sources []model.Source
	err := s.db.WithContext(ctx).Order("id ASC").Offset(offset).Limit(limit).Find(&sources).Error
	return sources, total, err
}

func (s *GormSourceStore) Create(ctx context.Context, src *model.Source) error {
	return s.db.WithContext(ctx).Create(src).Error
}

func (s *GormSourceStore) Save(ctx context.Context, src *model.Source) error {
	return s.db.WithContext(ctx).Save(src).Error
}

func (s *GormSourceStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.Source{}, id).Error
}

func (s *GormSourceStore) UpdateLastAlertAt(ctx context.Context, id int64, t time.Time) error {
	return s.db.WithContext(ctx).Model(&model.Source{}).Where("id = ?", id).
		Update("last_alert_at", t).Error
}

// GormAlertStore 用 GORM/PostgreSQL 实现 AlertStore
type GormAlertStore struct{ db *gorm.DB }

func NewGormAlertStore(db *gorm.DB) *GormAlertStore { return &GormAlertStore{db: db} }

func (s *GormAlertStore) Create(ctx context.Context, a *model.Alert) error {
	return s.db.WithContext(ctx).Create(a).Error
}

func (s *GormAlertStore) FindByID(ctx context.Context, id int64) (*model.Alert, error) {
	var a model.Alert
	err := s.db.WithContext(ctx).Take(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *GormAlertStore) UpdateDisposition(ctx context.Context, id int64, disposition string) error {
	return s.db.WithContext(ctx).Model(&model.Alert{}).Where("id = ?", id).
		Update("disposition", disposition).Error
}

func (s *GormAlertStore) FindLatestInWindow(ctx context.Context, fingerprint, status string,
	since time.Time, excludeID int64) (*model.Alert, error) {
	var a model.Alert
	err := s.db.WithContext(ctx).
		Where("fingerprint = ? AND status = ? AND received_at >= ? AND id <> ?",
			fingerprint, status, since, excludeID).
		Order("received_at DESC").
		Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *GormAlertStore) List(ctx context.Context, f AlertFilter) ([]model.Alert, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.Alert{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Severity != "" {
		q = q.Where("severity = ?", f.Severity)
	}
	if f.Alertname != "" {
		q = q.Where("alertname = ?", f.Alertname)
	}
	if f.Start != nil {
		q = q.Where("received_at >= ?", *f.Start)
	}
	if f.End != nil {
		q = q.Where("received_at <= ?", *f.End)
	}
	if f.ChannelID != nil {
		q = q.Where("id IN (?)", s.db.Model(&model.Delivery{}).
			Select("alert_id").Where("channel_id = ?", *f.ChannelID))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var alerts []model.Alert
	err := q.Order("received_at DESC, id DESC").Offset(f.Offset).Limit(f.Limit).Find(&alerts).Error
	return alerts, total, err
}

func (s *GormAlertStore) CountBySource(ctx context.Context, sourceID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.Alert{}).
		Where("source_id = ?", sourceID).Count(&n).Error
	return n, err
}

func (s *GormAlertStore) LatestRawPayload(ctx context.Context) (json.RawMessage, error) {
	var a model.Alert
	err := s.db.WithContext(ctx).Select("raw_payload").
		Order("received_at DESC, id DESC").Take(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return a.RawPayload, nil
}

// GormRuleStore 用 GORM/PostgreSQL 实现 RuleStore
type GormRuleStore struct{ db *gorm.DB }

func NewGormRuleStore(db *gorm.DB) *GormRuleStore { return &GormRuleStore{db: db} }

func (s *GormRuleStore) ListEnabledBySource(ctx context.Context, sourceID int64) ([]model.RoutingRule, error) {
	var rules []model.RoutingRule
	err := s.db.WithContext(ctx).
		Where("source_id = ? AND enabled", sourceID).
		Order("priority ASC, id ASC").
		Find(&rules).Error
	return rules, err
}

func (s *GormRuleStore) FindByID(ctx context.Context, id int64) (*model.RoutingRule, error) {
	var r model.RoutingRule
	err := s.db.WithContext(ctx).Take(&r, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *GormRuleStore) List(ctx context.Context, sourceID *int64, offset, limit int) ([]model.RoutingRule, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.RoutingRule{})
	if sourceID != nil {
		q = q.Where("source_id = ?", *sourceID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rules []model.RoutingRule
	err := q.Order("priority ASC, id ASC").Offset(offset).Limit(limit).Find(&rules).Error
	return rules, total, err
}

func (s *GormRuleStore) Create(ctx context.Context, r *model.RoutingRule) error {
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *GormRuleStore) Save(ctx context.Context, r *model.RoutingRule) error {
	return s.db.WithContext(ctx).Save(r).Error
}

func (s *GormRuleStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.RoutingRule{}, id).Error
}

func (s *GormRuleStore) CountBySource(ctx context.Context, sourceID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.RoutingRule{}).
		Where("source_id = ?", sourceID).Count(&n).Error
	return n, err
}

func (s *GormRuleStore) CountByChannel(ctx context.Context, channelID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.RoutingRule{}).
		Where("channel_id = ?", channelID).Count(&n).Error
	return n, err
}

// GormChannelStore 用 GORM/PostgreSQL 实现 ChannelStore
type GormChannelStore struct{ db *gorm.DB }

func NewGormChannelStore(db *gorm.DB) *GormChannelStore { return &GormChannelStore{db: db} }

func (s *GormChannelStore) FindByID(ctx context.Context, id int64) (*model.Channel, error) {
	var ch model.Channel
	err := s.db.WithContext(ctx).Take(&ch, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *GormChannelStore) FindByName(ctx context.Context, name string) (*model.Channel, error) {
	var ch model.Channel
	err := s.db.WithContext(ctx).Where("name = ?", name).Take(&ch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *GormChannelStore) List(ctx context.Context, offset, limit int) ([]model.Channel, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&model.Channel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var channels []model.Channel
	err := s.db.WithContext(ctx).Order("id ASC").Offset(offset).Limit(limit).Find(&channels).Error
	return channels, total, err
}

func (s *GormChannelStore) Create(ctx context.Context, ch *model.Channel) error {
	return s.db.WithContext(ctx).Create(ch).Error
}

func (s *GormChannelStore) Save(ctx context.Context, ch *model.Channel) error {
	return s.db.WithContext(ctx).Save(ch).Error
}

func (s *GormChannelStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.Channel{}, id).Error
}

func (s *GormChannelStore) CountByTemplate(ctx context.Context, templateID int64) (int64, error) {
	var n int64
	err := s.db.WithContext(ctx).Model(&model.Channel{}).
		Where("template_id = ?", templateID).Count(&n).Error
	return n, err
}

// GormTemplateStore 用 GORM/PostgreSQL 实现 TemplateStore
type GormTemplateStore struct{ db *gorm.DB }

func NewGormTemplateStore(db *gorm.DB) *GormTemplateStore { return &GormTemplateStore{db: db} }

func (s *GormTemplateStore) FindByID(ctx context.Context, id int64) (*model.Template, error) {
	var t model.Template
	err := s.db.WithContext(ctx).Take(&t, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *GormTemplateStore) FindBuiltin(ctx context.Context, channelType, name string) (*model.Template, error) {
	var t model.Template
	err := s.db.WithContext(ctx).
		Where("channel_type = ? AND name = ? AND is_builtin", channelType, name).
		Take(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *GormTemplateStore) FindByName(ctx context.Context, channelType, name string) (*model.Template, error) {
	var t model.Template
	err := s.db.WithContext(ctx).
		Where("channel_type = ? AND name = ?", channelType, name).
		Take(&t).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *GormTemplateStore) List(ctx context.Context, channelType string, offset, limit int) ([]model.Template, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.Template{})
	if channelType != "" {
		q = q.Where("channel_type = ?", channelType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var templates []model.Template
	err := q.Order("id ASC").Offset(offset).Limit(limit).Find(&templates).Error
	return templates, total, err
}

func (s *GormTemplateStore) Create(ctx context.Context, t *model.Template) error {
	return s.db.WithContext(ctx).Create(t).Error
}

func (s *GormTemplateStore) Save(ctx context.Context, t *model.Template) error {
	return s.db.WithContext(ctx).Save(t).Error
}

func (s *GormTemplateStore) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&model.Template{}, id).Error
}

// GormDeliveryStore 用 GORM/PostgreSQL 实现 DeliveryStore
type GormDeliveryStore struct{ db *gorm.DB }

func NewGormDeliveryStore(db *gorm.DB) *GormDeliveryStore { return &GormDeliveryStore{db: db} }

func (s *GormDeliveryStore) Create(ctx context.Context, d *model.Delivery) error {
	return s.db.WithContext(ctx).Create(d).Error
}

func (s *GormDeliveryStore) FindByID(ctx context.Context, id int64) (*model.Delivery, error) {
	var d model.Delivery
	err := s.db.WithContext(ctx).Take(&d, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *GormDeliveryStore) List(ctx context.Context, f DeliveryFilter) ([]model.Delivery, int64, error) {
	q := s.db.WithContext(ctx).Model(&model.Delivery{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.ChannelID != nil {
		q = q.Where("channel_id = ?", *f.ChannelID)
	}
	if f.Start != nil {
		q = q.Where("sent_at >= ?", *f.Start)
	}
	if f.End != nil {
		q = q.Where("sent_at <= ?", *f.End)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var deliveries []model.Delivery
	err := q.Order("sent_at DESC, id DESC").Offset(f.Offset).Limit(f.Limit).Find(&deliveries).Error
	return deliveries, total, err
}

func (s *GormDeliveryStore) ListByAlertID(ctx context.Context, alertID int64) ([]model.Delivery, error) {
	var deliveries []model.Delivery
	err := s.db.WithContext(ctx).Where("alert_id = ?", alertID).
		Order("id ASC").Find(&deliveries).Error
	return deliveries, err
}
