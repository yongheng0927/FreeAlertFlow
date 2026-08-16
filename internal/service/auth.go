// Package service 实现业务逻辑层 数据库访问通过一小组 store 接口隔离，
// 便于用内存 fake 做单元测试
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yongheng0927/fenghuo/internal/model"
	fafjwt "github.com/yongheng0927/fenghuo/internal/pkg/jwt"
	"github.com/yongheng0927/fenghuo/internal/pkg/password"
)

// 哨兵错误，由 handler 层映射为 HTTP 响应
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrInvalidToken       = errors.New("invalid refresh token")
	ErrTokenReuse         = errors.New("refresh token reuse detected, all sessions revoked")
	ErrNoLocalPassword    = errors.New("password login not available for this account")
)

// UserStore 抽象用户的持久化
type UserStore interface {
	// FindByUsername 在用户不存在时返回 (nil, nil)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	// FindByID 在用户不存在时返回 (nil, nil)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	Count(ctx context.Context) (int64, error)
	// List 返回一页用户（按 id 升序）和总数
	List(ctx context.Context, offset, limit int) ([]model.User, int64, error)
	Create(ctx context.Context, u *model.User) error
	UpdateLastLogin(ctx context.Context, id int64, t time.Time) error
	UpdatePassword(ctx context.Context, id int64, hash string) error
	// UpdateRoleAndStatus 更新角色和启用标志（用户管理）
	UpdateRoleAndStatus(ctx context.Context, id int64, role string, enabled bool) error
	// UpdateProfile 同步姓名/邮箱/头像（OAuth 登录，FR-5.3）
	UpdateProfile(ctx context.Context, id int64, name, email, avatarURL string) error
	// CountEnabledAdmins 统计启用状态的 admin 用户数
	CountEnabledAdmins(ctx context.Context) (int64, error)
	Delete(ctx context.Context, id int64) error
}

// RefreshTokenStore 抽象 refresh token 的持久化
type RefreshTokenStore interface {
	Create(ctx context.Context, t *model.RefreshToken) error
	// FindByHash 在 token 不存在时返回 (nil, nil)
	FindByHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	// Save 保存对已有 token 的修改
	Save(ctx context.Context, t *model.RefreshToken) error
	RevokeByHash(ctx context.Context, hash string) error
	RevokeAllForUser(ctx context.Context, userID int64) error
	// DeleteAllForUser 硬删除某用户的全部 token（删除用户时）
	DeleteAllForUser(ctx context.Context, userID int64) error
}

// TokenPair 在登录和刷新时返回
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // access token 有效期（秒）
}

// AuthService 实现登录、token 轮换、登出和修改密码
type AuthService struct {
	users  UserStore
	tokens RefreshTokenStore
	jwt    *fafjwt.Manager
	now    func() time.Time // 可注入，便于测试
}

// NewAuthService 创建使用真实时钟的 AuthService
func NewAuthService(users UserStore, tokens RefreshTokenStore, jwtMgr *fafjwt.Manager) *AuthService {
	return &AuthService{users: users, tokens: tokens, jwt: jwtMgr, now: time.Now}
}

// Login 用用户名密码认证并签发 token 对 纯 OAuth 用户（password_hash
// IS NULL）不能用密码登录
func (s *AuthService) Login(ctx context.Context, username, pw string) (*model.User, *TokenPair, error) {
	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	if user == nil || user.PasswordHash == nil {
		return nil, nil, ErrInvalidCredentials
	}
	if !password.Verify(*user.PasswordHash, pw) {
		return nil, nil, ErrInvalidCredentials
	}
	if !user.Enabled {
		return nil, nil, ErrAccountDisabled
	}
	if err := s.users.UpdateLastLogin(ctx, user.ID, s.now()); err != nil {
		return nil, nil, err
	}
	pair, err := s.issue(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

// Refresh 轮换 refresh token：旧 token 被吊销并由新 token 替代 拿着已轮换
// 的旧 token 再次请求视为重放攻击，吊销该用户的全部 refresh token
// （DATABASE_DESIGN §3）
func (s *AuthService) Refresh(ctx context.Context, plainToken string) (*TokenPair, error) {
	hash := fafjwt.HashRefreshToken(plainToken)
	rt, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if rt == nil {
		return nil, ErrInvalidToken
	}
	if rt.Revoked {
		if rt.ReplacedBy != "" {
			// 已轮换的 token 再次出现：判定为重放，全部吊销
			if err := s.tokens.RevokeAllForUser(ctx, rt.UserID); err != nil {
				return nil, err
			}
			return nil, ErrTokenReuse
		}
		// 登出或改密码导致的吊销，不是轮换
		return nil, ErrInvalidToken
	}
	if !s.now().Before(rt.ExpiresAt) {
		return nil, ErrInvalidToken
	}
	user, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidToken
	}
	if !user.Enabled {
		return nil, ErrAccountDisabled
	}

	pair, newHash, err := s.issueWithHash(ctx, user)
	if err != nil {
		return nil, err
	}
	rt.Revoked = true
	rt.ReplacedBy = newHash
	if err := s.tokens.Save(ctx, rt); err != nil {
		return nil, err
	}
	return pair, nil
}

// Logout 吊销指定的 refresh token 未知 token 直接忽略，保证登出幂等
func (s *AuthService) Logout(ctx context.Context, plainToken string) error {
	return s.tokens.RevokeByHash(ctx, fafjwt.HashRefreshToken(plainToken))
}

// ChangePassword 校验旧密码、设置新密码，并吊销该用户的全部 refresh
// token（强制所有会话重新登录）
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, oldPw, newPw string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrInvalidCredentials
	}
	if user.PasswordHash == nil {
		return ErrNoLocalPassword
	}
	if !password.Verify(*user.PasswordHash, oldPw) {
		return ErrInvalidCredentials
	}
	hash, err := password.Hash(newPw)
	if err != nil {
		return err
	}
	if err := s.users.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}
	return s.tokens.RevokeAllForUser(ctx, userID)
}

// BootstrapAdmin 在数据库没有任何用户且配置了
// FENGHUO_ADMIN_USER/FENGHUO_ADMIN_PASSWORD 时创建初始管理员，否则什么都不做
// 返回是否创建了用户
func (s *AuthService) BootstrapAdmin(ctx context.Context, username, pw string) (bool, error) {
	if username == "" || pw == "" {
		return false, nil
	}
	n, err := s.users.Count(ctx)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	hash, err := password.Hash(pw)
	if err != nil {
		return false, err
	}
	user := &model.User{
		Username:     username,
		PasswordHash: &hash,
		Name:         username,
		Role:         model.RoleAdmin,
		Enabled:      true,
	}
	if err := s.users.Create(ctx, user); err != nil {
		// 多副本同时首启：另一个实例已抢先创建，视为未创建而非失败
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, nil
		}
		return false, fmt.Errorf("create initial admin: %w", err)
	}
	return true, nil
}

// IssueTokens 为已完成认证的用户签发新 token 对（OAuth 流程在身份解析
// 后调用）
func (s *AuthService) IssueTokens(ctx context.Context, user *model.User) (*TokenPair, error) {
	return s.issue(ctx, user)
}

// issue 为用户创建新 token 对
func (s *AuthService) issue(ctx context.Context, user *model.User) (*TokenPair, error) {
	pair, _, err := s.issueWithHash(ctx, user)
	return pair, err
}

// issueWithHash 创建 token 对并同时返回存储用的 refresh token 哈希
// （Refresh 轮换时需要用它关联被替换掉的旧 token）
func (s *AuthService) issueWithHash(ctx context.Context, user *model.User) (*TokenPair, string, error) {
	access, err := s.jwt.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, "", err
	}
	refresh, hash, err := fafjwt.GenerateRefreshToken()
	if err != nil {
		return nil, "", err
	}
	rt := &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: hash,
		ExpiresAt: s.now().Add(s.jwt.RefreshTTL()),
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, "", err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
	}, hash, nil
}
