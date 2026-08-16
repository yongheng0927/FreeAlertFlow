package service

import (
	"context"

	"gorm.io/gorm"
)

// SettingKeyJWTSecret 是持久化的随机 JWT 密钥在 app_settings 中的键
const SettingKeyJWTSecret = "jwt.secret"

// LoadOrStoreSetting 读取 app_settings 中的键；不存在时写入给定值并返回
// 它 并发首启时以先写入者为准（ON CONFLICT DO NOTHING 后再读），保证
// 多副本拿到同一个值
func LoadOrStoreSetting(ctx context.Context, db *gorm.DB, key, value string) (string, error) {
	var stored string
	err := db.WithContext(ctx).Raw("SELECT value FROM app_settings WHERE key = ?", key).Scan(&stored).Error
	if err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
	}
	if err := db.WithContext(ctx).
		Raw("INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO NOTHING RETURNING value", key, value).
		Scan(&stored).Error; err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
	}
	// 并发下另一实例已写入：以其值为准
	if err := db.WithContext(ctx).Raw("SELECT value FROM app_settings WHERE key = ?", key).Scan(&stored).Error; err != nil {
		return "", err
	}
	return stored, nil
}
