-- 0001_init：创建 FreeAlertFlow V1 的全部 9 张表
-- 依据 DATABASE_DESIGN.md：BIGINT identity 主键、TIMESTAMPTZ、原生 BOOLEAN、
-- JSONB、BYTEA、VARCHAR 语义枚举（不使用 PG ENUM）、不使用物理外键

-- 公共触发器函数：每次 UPDATE 时自动维护 updated_at = now()
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 1. users
CREATE TABLE users (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL,
    password_hash VARCHAR(255),
    name          VARCHAR(64)  NOT NULL DEFAULT '',
    email         VARCHAR(128) NOT NULL DEFAULT '',
    avatar_url    VARCHAR(512) NOT NULL DEFAULT '',
    role          VARCHAR(16)  NOT NULL DEFAULT 'viewer',
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_users_username UNIQUE (username)
);
CREATE TRIGGER trg_users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 2. oauth_identities
CREATE TABLE oauth_identities (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id           BIGINT       NOT NULL,
    provider          VARCHAR(32)  NOT NULL,
    provider_user_id  VARCHAR(128) NOT NULL,
    provider_union_id VARCHAR(128) NOT NULL DEFAULT '',
    extra             JSONB,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_oauth_identities_provider_user UNIQUE (provider, provider_user_id)
);
CREATE INDEX idx_oauth_identities_user_id ON oauth_identities (user_id);
CREATE TRIGGER trg_oauth_identities_set_updated_at BEFORE UPDATE ON oauth_identities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 3. refresh_tokens
CREATE TABLE refresh_tokens (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     BIGINT       NOT NULL,
    token_hash  CHAR(64)     NOT NULL,
    expires_at  TIMESTAMPTZ  NOT NULL,
    revoked     BOOLEAN      NOT NULL DEFAULT FALSE,
    replaced_by CHAR(64)     NOT NULL DEFAULT '',
    client_ip   VARCHAR(45)  NOT NULL DEFAULT '',
    user_agent  VARCHAR(255) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash)
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE TRIGGER trg_refresh_tokens_set_updated_at BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 4. sources
CREATE TABLE sources (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          VARCHAR(128) NOT NULL,
    token         CHAR(32)     NOT NULL,
    description   VARCHAR(255) NOT NULL DEFAULT '',
    enabled       BOOLEAN      NOT NULL DEFAULT TRUE,
    last_alert_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_sources_name UNIQUE (name),
    CONSTRAINT uq_sources_token UNIQUE (token)
);
CREATE TRIGGER trg_sources_set_updated_at BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 5. templates
CREATE TABLE templates (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         VARCHAR(128) NOT NULL,
    channel_type VARCHAR(16)  NOT NULL,
    content      TEXT         NOT NULL,
    is_builtin   BOOLEAN      NOT NULL DEFAULT FALSE,
    remark       VARCHAR(255) NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_templates_channel_type_name UNIQUE (channel_type, name)
);
CREATE TRIGGER trg_templates_set_updated_at BEFORE UPDATE ON templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 6. channels
CREATE TABLE channels (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                  VARCHAR(128) NOT NULL,
    type                  VARCHAR(16)  NOT NULL DEFAULT 'feishu',
    webhook_url_encrypted BYTEA        NOT NULL,
    secret_encrypted      BYTEA,
    keyword               VARCHAR(64)  NOT NULL DEFAULT '',
    template_id           BIGINT,
    at_all                BOOLEAN      NOT NULL DEFAULT FALSE,
    extra                 JSONB,
    enabled               BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_channels_name UNIQUE (name)
);
CREATE INDEX idx_channels_template_id ON channels (template_id);
CREATE TRIGGER trg_channels_set_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 7. routing_rules
CREATE TABLE routing_rules (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id         BIGINT       NOT NULL,
    name              VARCHAR(128) NOT NULL DEFAULT '',
    priority          INT          NOT NULL DEFAULT 100,
    match_labels      JSONB        NOT NULL,
    channel_id        BIGINT       NOT NULL,
    continue_matching BOOLEAN      NOT NULL DEFAULT FALSE,
    enabled           BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_routing_rules_source_id ON routing_rules (source_id);
CREATE INDEX idx_routing_rules_channel_id ON routing_rules (channel_id);
CREATE TRIGGER trg_routing_rules_set_updated_at BEFORE UPDATE ON routing_rules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 8. alerts
CREATE TABLE alerts (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id    BIGINT       NOT NULL,
    fingerprint  CHAR(64)     NOT NULL,
    content_hash CHAR(64)     NOT NULL,
    status       VARCHAR(16)  NOT NULL,
    alertname    VARCHAR(255) NOT NULL DEFAULT '',
    severity     VARCHAR(32)  NOT NULL DEFAULT '',
    labels       JSONB        NOT NULL,
    annotations  JSONB        NOT NULL,
    starts_at    TIMESTAMPTZ  NOT NULL,
    ends_at      TIMESTAMPTZ,
    raw_payload  JSONB        NOT NULL,
    disposition  VARCHAR(16)  NOT NULL DEFAULT 'delivered',
    received_at  TIMESTAMPTZ  NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_alerts_source_id ON alerts (source_id);
CREATE INDEX idx_alerts_received_at ON alerts (received_at);
CREATE INDEX idx_alerts_fingerprint_status ON alerts (fingerprint, status);
CREATE INDEX idx_alerts_status_received_at ON alerts (status, received_at);
CREATE INDEX idx_alerts_severity_received_at ON alerts (severity, received_at);
CREATE TRIGGER trg_alerts_set_updated_at BEFORE UPDATE ON alerts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- 9. deliveries
CREATE TABLE deliveries (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    alert_id         BIGINT       NOT NULL,
    channel_id       BIGINT       NOT NULL,
    channel_name     VARCHAR(128) NOT NULL DEFAULT '',
    rule_id          BIGINT       NOT NULL DEFAULT 0,
    trigger_type     VARCHAR(16)  NOT NULL DEFAULT 'auto',
    attempts         INT          NOT NULL DEFAULT 1,
    status           VARCHAR(16)  NOT NULL,
    http_status      INT          NOT NULL DEFAULT 0,
    response_code    INT          NOT NULL DEFAULT 0,
    response_msg     VARCHAR(512) NOT NULL DEFAULT '',
    duration_ms      INT          NOT NULL DEFAULT 0,
    rendered_payload TEXT,
    sent_at          TIMESTAMPTZ  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_deliveries_alert_id ON deliveries (alert_id);
CREATE INDEX idx_deliveries_channel_id ON deliveries (channel_id);
CREATE INDEX idx_deliveries_status_sent_at ON deliveries (status, sent_at);
CREATE INDEX idx_deliveries_channel_id_sent_at ON deliveries (channel_id, sent_at);
CREATE TRIGGER trg_deliveries_set_updated_at BEFORE UPDATE ON deliveries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
