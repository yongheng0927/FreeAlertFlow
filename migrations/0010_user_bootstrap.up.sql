-- 0010_user_bootstrap：初始管理员改为首次启动后通过 Web API 设置（FR-5.1），
-- 不再依赖 FENGHUO_ADMIN_USER/FENGHUO_ADMIN_PASSWORD 环境变量。
-- is_bootstrap 标记引导创建的管理员：该账号受保护（不可降级/禁用/删除）；
-- 部分唯一索引保证全表至多一行 is_bootstrap = TRUE，并发 setup 时第二个
-- 请求会撞唯一冲突（23505），从而安全地映射为"setup 已完成"

ALTER TABLE users ADD COLUMN is_bootstrap BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX uq_users_single_bootstrap ON users (is_bootstrap) WHERE is_bootstrap;
