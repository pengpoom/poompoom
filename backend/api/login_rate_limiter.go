package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginRateLimiter struct {
	mu          sync.Mutex
	maxFailures int
	lockout     time.Duration
	now         func() time.Time
	entries     map[string]loginRateLimitEntry
}

type loginRateLimitEntry struct {
	failures  int
	lockedAt  time.Time
	updatedAt time.Time
}

func newLoginRateLimiter(maxFailures int, lockout time.Duration) *loginRateLimiter {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if lockout <= 0 {
		lockout = 15 * time.Minute
	}
	return &loginRateLimiter{
		maxFailures: maxFailures,
		lockout:     lockout,
		now:         time.Now,
		entries:     map[string]loginRateLimitEntry{},
	}
}

func (l *loginRateLimiter) allow(key string) bool {
	key = strings.TrimSpace(key)
	if l == nil || key == "" {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok || entry.lockedAt.IsZero() {
		return true
	}
	if now.Sub(entry.lockedAt) >= l.lockout {
		delete(l.entries, key)
		return true
	}
	return false
}

func (l *loginRateLimiter) recordFailure(key string) {
	key = strings.TrimSpace(key)
	if l == nil || key == "" {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if !entry.lockedAt.IsZero() && now.Sub(entry.lockedAt) >= l.lockout {
		entry = loginRateLimitEntry{}
	}
	entry.failures++
	entry.updatedAt = now
	if entry.failures >= l.maxFailures {
		entry.lockedAt = now
	}
	l.entries[key] = entry
}

func (l *loginRateLimiter) recordSuccess(key string) {
	key = strings.TrimSpace(key)
	if l == nil || key == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func loginRateLimitKey(r *http.Request, username string) string {
	return clientIP(r) + "|" + strings.ToLower(strings.TrimSpace(username))
}

func clientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	for _, header := range []string{"CF-Connecting-IP", "X-Real-IP"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return value
		}
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if first := strings.TrimSpace(parts[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
