package crypto

import (
	"bytes"
	"testing"
)

func testKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	c, err := New(testKey())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plain := []byte("https://open.feishu.cn/open-apis/bot/v2/hook/secret-token")
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(enc, plain) {
		t.Fatal("ciphertext contains plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(dec, plain) {
		t.Fatalf("round trip mismatch: got %q", dec)
	}
}

func TestEncryptUsesRandomNonce(t *testing.T) {
	c, _ := New(testKey())
	a, _ := c.Encrypt([]byte("same"))
	b, _ := c.Encrypt([]byte("same"))
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same plaintext must differ (random nonce)")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	c1, _ := New(testKey())
	c2, _ := New([]byte("abcdef0123456789abcdef0123456789"))
	enc, _ := c1.Encrypt([]byte("secret"))
	if _, err := c2.Decrypt(enc); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestDecryptTamperedFails(t *testing.T) {
	c, _ := New(testKey())
	enc, _ := c.Encrypt([]byte("secret"))
	enc[len(enc)-1] ^= 0xff
	if _, err := c.Decrypt(enc); err == nil {
		t.Fatal("decrypt of tampered ciphertext must fail")
	}
}

func TestDecryptTooShortFails(t *testing.T) {
	c, _ := New(testKey())
	if _, err := c.Decrypt([]byte("x")); err == nil {
		t.Fatal("decrypt of short input must fail")
	}
}

func TestNewRejectsBadKeyLength(t *testing.T) {
	if _, err := New([]byte("short")); err == nil {
		t.Fatal("New must reject non-32-byte keys")
	}
}
