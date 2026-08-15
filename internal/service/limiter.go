package service

import (
	"sync"
	"time"
)

// LoginLimiter 是基于内存的按 IP 登录限流器（NFR-1）：window 内失败
// maxFails 次后，该 IP 被锁定 lockTime
type LoginLimiter struct {
	mu       sync.Mutex
	maxFails int
	window   time.Duration
	lockTime time.Duration
	entries  map[string]*limiterEntry
	now      func() time.Time // 可注入，便于测试
}

type limiterEntry struct {
	fails       []time.Time
	lockedUntil time.Time
}

// NewLoginLimiter 创建限流器：window 内失败 maxFails 次即锁定 IP lockTime
func NewLoginLimiter(maxFails int, window, lockTime time.Duration) *LoginLimiter {
	return &LoginLimiter{
		maxFails: maxFails,
		window:   window,
		lockTime: lockTime,
		entries:  make(map[string]*limiterEntry),
		now:      time.Now,
	}
}

// Locked 报告该 IP 当前是否处于锁定状态
func (l *LoginLimiter) Locked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[ip]
	return e != nil && l.now().Before(e.lockedUntil)
}

// Fail 记录一次登录失败，滑动窗口内失败次数达到上限即锁定该 IP
func (l *LoginLimiter) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e := l.entries[ip]
	if e == nil {
		e = &limiterEntry{}
		l.entries[ip] = e
	}
	// 只保留滑动窗口内的失败记录
	kept := e.fails[:0]
	for _, t := range e.fails {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	e.fails = append(kept, now)
	if len(e.fails) >= l.maxFails {
		e.lockedUntil = now.Add(l.lockTime)
		e.fails = e.fails[:0]
	}
}

// Reset 清除该 IP 的失败状态（登录成功时调用）
func (l *LoginLimiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}
