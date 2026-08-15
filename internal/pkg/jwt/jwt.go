// Package jwt 签发与校验 JWT access token，并生成不透明（opaque）的
// refresh token（数据库中只保存其 SHA-256 哈希）
package jwt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	gjwt "github.com/golang-jwt/jwt/v5"
)

// Claims 在 access token 中携带用户身份
type Claims struct {
	Role string `json:"role"`
	gjwt.RegisteredClaims
}

// Manager 负责签发和校验 access token
type Manager struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager 创建 Manager，secret 不能为空
func NewManager(secret []byte, accessTTL, refreshTTL time.Duration) *Manager {
	return &Manager{secret: secret, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

// AccessTTL 返回配置的 access token 有效期
func (m *Manager) AccessTTL() time.Duration { return m.accessTTL }

// RefreshTTL 返回配置的 refresh token 有效期
func (m *Manager) RefreshTTL() time.Duration { return m.refreshTTL }

// GenerateAccessToken 为指定用户签发新的 access token
func (m *Manager) GenerateAccessToken(userID int64, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: gjwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  gjwt.NewNumericDate(now),
			ExpiresAt: gjwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	return gjwt.NewWithClaims(gjwt.SigningMethodHS256, claims).SignedString(m.secret)
}

// ParseAccessToken 校验 token 并返回其中的 claims
func (m *Manager) ParseAccessToken(tokenStr string) (*Claims, error) {
	var claims Claims
	_, err := gjwt.ParseWithClaims(tokenStr, &claims, func(t *gjwt.Token) (any, error) {
		if _, ok := t.Method.(*gjwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return &claims, nil
}

// UserID 从已校验的 claims 中提取用户 id
func (c *Claims) UserID() (int64, error) {
	if c.Subject == "" {
		return 0, errors.New("token has no subject")
	}
	return strconv.ParseInt(c.Subject, 10, 64)
}

// GenerateRefreshToken 生成新的不透明 refresh token 及其存储用哈希
func GenerateRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken 返回 token 的 SHA-256 十六进制值；数据库只保存该值
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
