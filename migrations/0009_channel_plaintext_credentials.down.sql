-- 0009 回滚：重建加密列（明文数据随之丢弃）

ALTER TABLE channels
    DROP COLUMN webhook_url,
    DROP COLUMN secret,
    ADD COLUMN webhook_url_encrypted BYTEA NOT NULL DEFAULT '',
    ADD COLUMN secret_encrypted      BYTEA;
