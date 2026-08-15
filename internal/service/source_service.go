package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// 管理类 API 共享的哨兵错误
var (
	ErrNotFound      = errors.New("not found")
	ErrDuplicateName = errors.New("name already exists")
	ErrReferenced    = errors.New("resource is still referenced")
	ErrValidation    = errors.New("validation failed")
)

// validationErr 构造带字段信息的 400 错误
func validationErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}

// referencedErr 构造 409 错误
func referencedErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrReferenced, fmt.Sprintf(format, args...))
}

// GenerateSourceToken 生成 32 位随机 hex 的 webhook token（NFR-1）
func GenerateSourceToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SourceService 实现接入源管理（CRUD + token 轮换），删除时按
// DATABASE_DESIGN 的守卫规则校验（仍被规则/告警引用则拒绝）
type SourceService struct {
	sources SourceStore
	rules   RuleStore
	alerts  AlertStore
}

// NewSourceService 创建 SourceService
func NewSourceService(sources SourceStore, rules RuleStore, alerts AlertStore) *SourceService {
	return &SourceService{sources: sources, rules: rules, alerts: alerts}
}

// Create 创建接入源并生成 webhook token
func (s *SourceService) Create(ctx context.Context, name, description string) (*model.Source, error) {
	if name == "" {
		return nil, validationErr("name is required")
	}
	existing, err := s.sources.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, name)
	}
	token, err := GenerateSourceToken()
	if err != nil {
		return nil, err
	}
	src := &model.Source{Name: name, Token: token, Description: description, Enabled: true}
	if err := s.sources.Create(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

// SourcePatch 承载可选的更新字段，nil 表示保持原值
type SourcePatch struct {
	Name        *string
	Description *string
	Enabled     *bool
}

// Update 对接入源应用补丁
func (s *SourceService) Update(ctx context.Context, id int64, p SourcePatch) (*model.Source, error) {
	src, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, fmt.Errorf("%w: source %d", ErrNotFound, id)
	}
	if p.Name != nil {
		if *p.Name == "" {
			return nil, validationErr("name must not be empty")
		}
		if *p.Name != src.Name {
			existing, err := s.sources.FindByName(ctx, *p.Name)
			if err != nil {
				return nil, err
			}
			if existing != nil {
				return nil, fmt.Errorf("%w: %q", ErrDuplicateName, *p.Name)
			}
		}
		src.Name = *p.Name
	}
	if p.Description != nil {
		src.Description = *p.Description
	}
	if p.Enabled != nil {
		src.Enabled = *p.Enabled
	}
	if err := s.sources.Save(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

// Delete 删除接入源，仍有规则或告警引用时拒绝（应用层引用检查 + 硬删除，
// DB 设计 §11）
func (s *SourceService) Delete(ctx context.Context, id int64) error {
	src, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if src == nil {
		return fmt.Errorf("%w: source %d", ErrNotFound, id)
	}
	n, err := s.rules.CountBySource(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return referencedErr("source has %d routing rules, delete them first", n)
	}
	n, err = s.alerts.CountBySource(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return referencedErr("source has %d alerts; they expire with the retention policy", n)
	}
	return s.sources.Delete(ctx, id)
}

// RotateToken 签发新的 webhook token，旧 token 立即失效
func (s *SourceService) RotateToken(ctx context.Context, id int64) (*model.Source, error) {
	src, err := s.sources.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return nil, fmt.Errorf("%w: source %d", ErrNotFound, id)
	}
	token, err := GenerateSourceToken()
	if err != nil {
		return nil, err
	}
	src.Token = token
	if err := s.sources.Save(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}
