// Package crypto 为敏感字段（channel 的 webhook URL、签名密钥）提供
// AES-256-GCM 加密，输出为 nonce||ciphertext 的二进制形式，存储在 BYTEA 列中
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// KeySize 是要求的密钥长度（字节数）（AES-256）
const KeySize = 32

// Cipher 包装一个 AEAD，用于加解密操作
type Cipher struct {
	aead cipher.AEAD
}

// New 用 32 字节密钥（来自 FAF_SECRET_KEY）创建 Cipher
func New(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("key must be exactly %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 使用随机 nonce 加密，返回 nonce||ciphertext
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt 是 Encrypt 的逆操作
func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(data) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := data[:ns], data[ns:]
	return c.aead.Open(nil, nonce, ct, nil)
}
