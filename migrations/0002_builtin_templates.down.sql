-- 0002_builtin_templates down：移除 0002 写入的内置模板
DELETE FROM templates
WHERE channel_type = 'feishu' AND is_builtin
  AND name IN ('critical-card', 'warning-card', 'resolved-card', 'plain-text');
