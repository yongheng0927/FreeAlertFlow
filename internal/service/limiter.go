package service

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// LoginLimiter 是按 IP 的登录限流器（NFR-1），状态存于 login_attempts 表
// （迁移 0008），多副本共享计数与锁定状态。语义为固定窗口（原内存版是
// 滑动窗口，迁库后的常规取舍，Grafana 同款思路）：window_start 起 window
// 内失败 maxFails 次，该 IP 被锁定 lockTime；now - window_start >= window
// 时窗口重置重新计数
type LoginLimiter struct {
	store    LoginAttemptStore
	maxFails int
	window   time.Duration
	lockTime time.Duration
	now      func() time.Time // 可注入，便于测试
}

// loginAttempt 是 login_attempts 表一行的内存表示
type loginAttempt struct {
	Fails       int
	WindowStart time.Time
	// LockedUntil 为 nil 表示从未锁定（对应 DB 的 '-infinity' 默认值）；
	// 非 nil 但已过期的值视为未锁定，由时间比较自然判定
	LockedUntil *time.Time
}

// locked 报告该状态在 now 时刻是否处于锁定期内
func (a loginAttempt) locked(now time.Time) bool {
	return a.LockedUntil != nil && now.Before(*a.LockedUntil)
}

// applyFailure 是纯函数：在当前状态上计入一次失败并返回新状态。
// 锁定期内的失败不再计数，避免并发请求在锁定期间反复累计、延长锁定
func applyFailure(a loginAttempt, now time.Time, maxFails int, window, lockTime time.Duration) loginAttempt {
	if a.locked(now) {
		return a
	}
	// 固定窗口：窗口过期则从 now 起重新开窗计数
	if a.WindowStart.IsZero() || now.Sub(a.WindowStart) >= window {
		a = loginAttempt{WindowStart: now}
	}
	a.Fails++
	if a.Fails >= maxFails {
		until := now.Add(lockTime)
		a.LockedUntil = &until
		a.Fails = 0
	}
	return a
}

// LoginAttemptStore 抽象 login_attempts 表的读写，只做事务管道，不含
// 窗口/锁定判定逻辑（判定见上面的纯函数，便于单测）
type LoginAttemptStore interface {
	// Read 返回该 IP 当前的状态，无记录时返回零值
	Read(ctx context.Context, ip string) (loginAttempt, error)
	// Update 在事务内先确保行存在，再 SELECT ... FOR UPDATE 取行锁，
	// 把当前行交给 fn 计算新状态后写回。同一 IP 的读改写被行锁串行化
	// （多副本/并发下计数不丢），不同 IP 互不阻塞
	Update(ctx context.Context, ip string, now time.Time, fn func(loginAttempt) loginAttempt) error
	// Reset 删除该 IP 的行（登录成功时调用）
	Reset(ctx context.Context, ip string) error
}

// NewLoginLimiter 创建限流器：window 内失败 maxFails 次即锁定 IP lockTime
func NewLoginLimiter(store LoginAttemptStore, maxFails int, window, lockTime time.Duration) *LoginLimiter {
	return &LoginLimiter{
		store:    store,
		maxFails: maxFails,
		window:   window,
		lockTime: lockTime,
		now:      time.Now,
	}
}

// Locked 报告该 IP 当前是否处于锁定状态
func (l *LoginLimiter) Locked(ctx context.Context, ip string) (bool, error) {
	a, err := l.store.Read(ctx, ip)
	if err != nil {
		return false, err
	}
	return a.locked(l.now()), nil
}

// Fail 记录一次登录失败，固定窗口内失败次数达到上限即锁定该 IP
func (l *LoginLimiter) Fail(ctx context.Context, ip string) error {
	now := l.now()
	return l.store.Update(ctx, ip, now, func(a loginAttempt) loginAttempt {
		return applyFailure(a, now, l.maxFails, l.window, l.lockTime)
	})
}

// Reset 清除该 IP 的失败状态（登录成功时调用）
func (l *LoginLimiter) Reset(ctx context.Context, ip string) error {
	return l.store.Reset(ctx, ip)
}

// GormLoginAttemptStore 用 GORM/PostgreSQL 实现 LoginAttemptStore。
// 失败/锁定行随时间过期但行不删（体积按 distinct IP 增长，可接受），
// 不做定期清理任务
type GormLoginAttemptStore struct {
	db *gorm.DB
}

// NewGormLoginAttemptStore 创建 GormLoginAttemptStore
func NewGormLoginAttemptStore(db *gorm.DB) *GormLoginAttemptStore {
	return &GormLoginAttemptStore{db: db}
}

// lockedUntil 列的 '-infinity' 默认值无法直接扫描进 time.Time，读出时
// 统一映射为 NULL（即 loginAttempt.LockedUntil 的 nil）
const loginAttemptSelect = `
	SELECT fails, window_start,
	       CASE WHEN locked_until = '-infinity'::timestamptz THEN NULL ELSE locked_until END AS locked_until
	FROM login_attempts WHERE ip = ?`

func (s *GormLoginAttemptStore) Read(ctx context.Context, ip string) (loginAttempt, error) {
	var a loginAttempt
	if err := s.db.WithContext(ctx).Raw(loginAttemptSelect, ip).Scan(&a).Error; err != nil {
		return loginAttempt{}, err
	}
	return a, nil
}

func (s *GormLoginAttemptStore) Update(ctx context.Context, ip string, now time.Time, fn func(loginAttempt) loginAttempt) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 并发首访同一 IP 时先写入者建行，后到者走 ON CONFLICT 不报错
		if err := tx.Exec(
			"INSERT INTO login_attempts (ip, window_start) VALUES (?, ?) ON CONFLICT (ip) DO NOTHING",
			ip, now).Error; err != nil {
			return err
		}
		var a loginAttempt
		if err := tx.Raw(loginAttemptSelect+" FOR UPDATE", ip).Scan(&a).Error; err != nil {
			return err
		}
		next := fn(a)
		// LockedUntil 为 nil 时写回 '-infinity'（列默认值语义：从未锁定）
		return tx.Exec(
			`UPDATE login_attempts SET fails = ?, window_start = ?,
			 locked_until = COALESCE(?::timestamptz, '-infinity'::timestamptz) WHERE ip = ?`,
			next.Fails, next.WindowStart, next.LockedUntil, ip).Error
	})
}

func (s *GormLoginAttemptStore) Reset(ctx context.Context, ip string) error {
	return s.db.WithContext(ctx).Exec("DELETE FROM login_attempts WHERE ip = ?", ip).Error
}
