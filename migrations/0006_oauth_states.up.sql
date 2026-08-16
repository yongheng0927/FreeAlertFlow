-- 0006_oauth_states：OAuth 登录的 CSRF state 从内存迁至数据库，
-- 多副本部署时签发与回调可能落在不同实例，内存态会导致登录间歇性失败

CREATE TABLE oauth_states (
    state       TEXT        NOT NULL PRIMARY KEY,
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_oauth_states_expires_at ON oauth_states (expires_at);
