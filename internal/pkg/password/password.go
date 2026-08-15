// Package password 使用 bcrypt（cost 12）对密码进行哈希与校验
package password

import "golang.org/x/crypto/bcrypt"

// Cost 是 bcrypt 的工作因子（NFR-1 要求 >= 10）
const Cost = 12

// Hash 返回 pw 的 bcrypt 哈希
func Hash(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), Cost)
	return string(b), err
}

// Verify 报告 pw 是否与 bcrypt 哈希匹配
func Verify(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
