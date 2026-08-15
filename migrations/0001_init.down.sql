-- 0001_init down：删除所有表以及公共触发器函数
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS routing_rules;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS oauth_identities;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();
