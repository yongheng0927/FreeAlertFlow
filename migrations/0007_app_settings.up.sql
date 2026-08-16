-- 0007_app_settings：通用键值设置表，首个用途是持久化启动时随机生成的
-- JWT 密钥（未显式配置 FENGHUO_JWT_SECRET 时），避免重启/多副本下
-- 各实例密钥不一致导致 token 失效

CREATE TABLE app_settings (
    key         TEXT NOT NULL PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
