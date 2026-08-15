package jwt

import (
	"strings"
	"testing"
	"time"
)

var testSecret = []byte("test-jwt-secret-key-0123456789ab")

func TestAccessTokenRoundTrip(t *testing.T) {
	m := NewManager(testSecret, 2*time.Hour, 7*24*time.Hour)
	tok, err := m.GenerateAccessToken(42, "editor")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseAccessToken(tok)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	id, err := claims.UserID()
	if err != nil {
		t.Fatalf("UserID: %v", err)
	}
	if id != 42 {
		t.Fatalf("user id = %d, want 42", id)
	}
	if claims.Role != "editor" {
		t.Fatalf("role = %q, want editor", claims.Role)
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	m := NewManager(testSecret, -time.Second, time.Hour) // 已过期
	tok, err := m.GenerateAccessToken(1, "viewer")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if _, err := m.ParseAccessToken(tok); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestTamperedTokenRejected(t *testing.T) {
	m := NewManager(testSecret, time.Hour, time.Hour)
	tok, _ := m.GenerateAccessToken(1, "viewer")
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format")
	}
	// 篡改 payload 中的一个字符：签名将不再匹配
	payload := []byte(parts[1])
	if payload[0] == 'A' {
		payload[0] = 'B'
	} else {
		payload[0] = 'A'
	}
	parts[1] = string(payload)
	if _, err := m.ParseAccessToken(strings.Join(parts, ".")); err == nil {
		t.Fatal("tampered token must be rejected")
	}
}

func TestWrongSecretRejected(t *testing.T) {
	m1 := NewManager(testSecret, time.Hour, time.Hour)
	m2 := NewManager([]byte("another-secret-key-0123456789abc"), time.Hour, time.Hour)
	tok, _ := m1.GenerateAccessToken(1, "viewer")
	if _, err := m2.ParseAccessToken(tok); err == nil {
		t.Fatal("token signed with another secret must be rejected")
	}
}

func TestRefreshTokenHash(t *testing.T) {
	tok1, hash1, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	tok2, hash2, _ := GenerateRefreshToken()
	if tok1 == tok2 {
		t.Fatal("refresh tokens must be unique")
	}
	if len(hash1) != 64 || len(hash2) != 64 {
		t.Fatal("hash must be 64 hex chars (SHA-256)")
	}
	if HashRefreshToken(tok1) != hash1 {
		t.Fatal("HashRefreshToken must reproduce the stored hash")
	}
	if HashRefreshToken(tok2) == hash1 {
		t.Fatal("different tokens must have different hashes")
	}
}
