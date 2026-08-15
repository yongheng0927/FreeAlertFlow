package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yongheng0927/FreeAlertFlow/internal/model"
)

// RuleService 实现路由规则管理（FR-3）
type RuleService struct {
	rules    RuleStore
	sources  SourceStore
	channels ChannelStore
}

// NewRuleService 创建 RuleService
func NewRuleService(rules RuleStore, sources SourceStore, channels ChannelStore) *RuleService {
	return &RuleService{rules: rules, sources: sources, channels: channels}
}

// RuleInput 承载路由规则的创建/更新字段
type RuleInput struct {
	SourceID         int64
	Name             string
	Priority         int
	MatchLabels      json.RawMessage
	ChannelID        int64
	ContinueMatching bool
	Enabled          bool
}

// validateInput 校验引用关系和 match_labels 的格式
func (s *RuleService) validateInput(ctx context.Context, in RuleInput) error {
	if in.SourceID == 0 {
		return validationErr("source_id is required")
	}
	src, err := s.sources.FindByID(ctx, in.SourceID)
	if err != nil {
		return err
	}
	if src == nil {
		return validationErr("source %d does not exist", in.SourceID)
	}
	if in.ChannelID == 0 {
		return validationErr("channel_id is required")
	}
	ch, err := s.channels.FindByID(ctx, in.ChannelID)
	if err != nil {
		return err
	}
	if ch == nil {
		return validationErr("channel %d does not exist", in.ChannelID)
	}
	if len(in.MatchLabels) == 0 {
		return validationErr("match_labels is required (use {} for the default rule)")
	}
	var m map[string]string
	if err := json.Unmarshal(in.MatchLabels, &m); err != nil || m == nil {
		return validationErr("match_labels must be a JSON object of string key=value pairs")
	}
	return nil
}

// Create 校验并保存规则
func (s *RuleService) Create(ctx context.Context, in RuleInput) (*model.RoutingRule, error) {
	if err := s.validateInput(ctx, in); err != nil {
		return nil, err
	}
	if in.Priority == 0 {
		in.Priority = 100 // 未指定时用数据库默认值
	}
	r := &model.RoutingRule{
		SourceID:         in.SourceID,
		Name:             in.Name,
		Priority:         in.Priority,
		MatchLabels:      in.MatchLabels,
		ChannelID:        in.ChannelID,
		ContinueMatching: in.ContinueMatching,
		Enabled:          in.Enabled,
	}
	if err := s.rules.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Update 替换规则的可编辑字段
func (s *RuleService) Update(ctx context.Context, id int64, in RuleInput) (*model.RoutingRule, error) {
	r, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("%w: rule %d", ErrNotFound, id)
	}
	if err := s.validateInput(ctx, in); err != nil {
		return nil, err
	}
	r.SourceID = in.SourceID
	r.Name = in.Name
	r.Priority = in.Priority
	r.MatchLabels = in.MatchLabels
	r.ChannelID = in.ChannelID
	r.ContinueMatching = in.ContinueMatching
	r.Enabled = in.Enabled
	if err := s.rules.Save(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete 删除规则（没有其他资源引用规则，无需检查）
func (s *RuleService) Delete(ctx context.Context, id int64) error {
	r, err := s.rules.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if r == nil {
		return fmt.Errorf("%w: rule %d", ErrNotFound, id)
	}
	return s.rules.Delete(ctx, id)
}
