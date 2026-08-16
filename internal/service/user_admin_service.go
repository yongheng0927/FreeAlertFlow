package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yongheng0927/fenghuo/internal/model"
	"github.com/yongheng0927/fenghuo/internal/pkg/password"
)

// 用户管理的守卫错误（FR-5.4，仅 admin）
var (
	ErrLastAdmin        = errors.New("cannot disable, demote or delete the last admin")
	ErrCannotDeleteSelf = errors.New("cannot delete your own account")
	// ErrInitialAdmin 表示试图降级、禁用或删除初始管理员（FR-5.1 创建的账号）
	ErrInitialAdmin = errors.New("the initial admin account cannot be demoted, disabled or deleted")
)

// UserAdminService 实现仅 admin 可用的用户管理
type UserAdminService struct {
	users  UserStore
	tokens RefreshTokenStore
	oauth  OAuthIdentityStore
	// initialAdmin 是初始管理员的用户名（FENGHUO_ADMIN_USER），该账号受保护
	initialAdmin string
}

// NewUserAdminService 创建 UserAdminService
func NewUserAdminService(users UserStore, tokens RefreshTokenStore, oauth OAuthIdentityStore, initialAdmin string) *UserAdminService {
	return &UserAdminService{users: users, tokens: tokens, oauth: oauth, initialAdmin: initialAdmin}
}

// IsInitial 报告用户是否为受保护的初始管理员
func (s *UserAdminService) IsInitial(u *model.User) bool {
	return s.initialAdmin != "" && u.Username == s.initialAdmin
}

// ValidRole 报告 role 是否为 viewer/editor/admin 之一
func ValidRole(role string) bool {
	return role == model.RoleViewer || role == model.RoleEditor || role == model.RoleAdmin
}

// Create 由 admin 创建本地账号（FR-5.4）：用户名唯一，初始密码至少 8 位
// （与修改密码的策略一致），角色缺省 viewer
func (s *UserAdminService) Create(ctx context.Context, username, pw, role string) (*model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, validationErr("username is required")
	}
	if len(username) > 64 {
		return nil, validationErr("username must be at most 64 characters")
	}
	if len(pw) < 8 {
		return nil, validationErr("password must be at least 8 characters")
	}
	if role == "" {
		role = model.RoleViewer
	}
	if !ValidRole(role) {
		return nil, validationErr("role must be viewer, editor or admin")
	}
	existing, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: %q", ErrDuplicateName, username)
	}
	hash, err := password.Hash(pw)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:     username,
		PasswordHash: &hash,
		Role:         role,
		Enabled:      true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
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
	// 初始管理员不能被改角色或禁用
	if s.IsInitial(target) && ((role != nil && *role != target.Role) || (enabled != nil && !*enabled)) {
		return nil, ErrInitialAdmin
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

// ResetPassword 由初始管理员重置本地用户的密码（FR-5.4）：目标为纯 OAuth
// 账号（无本地密码）时拒绝；成功后吊销目标的全部 refresh token
func (s *UserAdminService) ResetPassword(ctx context.Context, actorID, targetID int64, newPw string) error {
	actor, err := s.users.FindByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor == nil || !s.IsInitial(actor) {
		return fmt.Errorf("%w: only the initial admin can reset passwords", ErrValidation)
	}
	if len(newPw) < 8 {
		return validationErr("password must be at least 8 characters")
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("%w: user %d", ErrNotFound, targetID)
	}
	if target.PasswordHash == nil {
		return validationErr("oauth-only account has no local password to reset")
	}
	hash, err := password.Hash(newPw)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, targetID, hash); err != nil {
		return err
	}
	return s.tokens.RevokeAllForUser(ctx, targetID)
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
	// 初始管理员不能被删除
	if s.IsInitial(target) {
		return ErrInitialAdmin
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
