-- 0009_channel_plaintext_credentials：渠道凭证改为明文存储
-- 不再使用 AES-GCM 加密列（secret_key 配置随之移除）；存量加密数据随列
-- 一并丢弃，渠道在 UI 重建

ALTER TABLE channels
    DROP COLUMN webhook_url_encrypted,
    DROP COLUMN secret_encrypted,
    ADD COLUMN webhook_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN secret      TEXT NOT NULL DEFAULT '';
