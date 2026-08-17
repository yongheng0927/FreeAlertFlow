-- 0008_login_attempts：登录限流（NFR-1）从内存迁至数据库共享存储，
-- 修复多副本下防爆破阈值按副本数放大、锁定状态不共享的问题。
-- 每行一个 IP：固定窗口 window_start 起累计 fails，达上限后 locked_until
-- 置为锁定截止时间；'-infinity' 表示从未锁定

CREATE TABLE login_attempts (
    ip           TEXT        NOT NULL PRIMARY KEY,
    fails        INT         NOT NULL DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ NOT NULL DEFAULT '-infinity'
);
