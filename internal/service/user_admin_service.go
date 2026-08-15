package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/yongheng0927/fenghuo/internal/model"
)

// 用户管理的守卫错误（FR-5.4，仅 admin）
var (
	ErrLastAdmin        = errors.New("cannot disable, demote or delete the last admin")
	ErrCannotDeleteSelf = errors.New("cannot delete your own account")
)

// UserAdminService 实现仅 admin 可用的用户管理
type UserAdminService struct {
	users  UserStore
	tokens RefreshTokenStore
	oauth  OAuthIdentityStore
}

// NewUserAdminService 创建 UserAdminService
func NewUserAdminService(users UserStore, tokens RefreshTokenStore, oauth OAuthIdentityStore) *UserAdminService {
	return &UserAdminService{users: users, tokens: tokens, oauth: oauth}
}

// ValidRole 报告 role 是否为 viewer/editor/admin 之一
func ValidRole(role string) bool {
	return role == model.RoleViewer || role == model.RoleEditor || role == model.RoleAdmin
}

// Update 修改用户的角色和/或启用状态 禁用的同时吊销该用户全部 refresh
// token（与 JWT 中间件的 enabled 检查保持一致） 最后一个启用的 admin 不能
// 被降级或禁用
func (s *UserAdminService) Update(ctx context.Context, targetID int64,
	role *string, enabled *bool) (*model.User, error) {
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, fmt.Errorf("%w: user %d", ErrNotFound, targetID)
	}
	newRole, newEnabled := target.Role, target.Enabled
	if role != nil {
		if !ValidRole(*role) {
			return nil, validationErr("role must be viewer, editor or admin")
		}
		newRole = *role
	}
	if enabled != nil {
		newEnabled = *enabled
	}
	losingAdmin := target.Role == model.RoleAdmin && target.Enabled &&
		(newRole != model.RoleAdmin || !newEnabled)
	if losingAdmin {
		n, err := s.users.CountEnabledAdmins(ctx)
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, ErrLastAdmin
		}
	}
	if err := s.users.UpdateRoleAndStatus(ctx, targetID, newRole, newEnabled); err != nil {
		return nil, err
	}
	if enabled != nil && !*enabled {
		if err := s.tokens.RevokeAllForUser(ctx, targetID); err != nil {
			return nil, err
		}
	}
	target.Role = newRole
	target.Enabled = newEnabled
	return target, nil
}

// Delete 删除用户及其 refresh token 和 OAuth 身份绑定（硬删除，DB 设计
// §11） 拒绝删除自己和最后一个 admin
func (s *UserAdminService) Delete(ctx context.Context, actorID, targetID int64) error {
	if actorID == targetID {
		return ErrCannotDeleteSelf
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("%w: user %d", ErrNotFound, targetID)
	}
	if target.Role == model.RoleAdmin && target.Enabled {
		n, err := s.users.CountEnabledAdmins(ctx)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	if err := s.tokens.DeleteAllForUser(ctx, targetID); err != nil {
		return err
	}
	if err := s.oauth.DeleteAllForUser(ctx, targetID); err != nil {
		return err
	}
	return s.users.Delete(ctx, targetID)
}
