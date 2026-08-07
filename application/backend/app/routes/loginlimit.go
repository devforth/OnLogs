package routes

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	freeLoginAttempts = 5
	maxLoginBackoff   = 15 * time.Minute
)

type loginAttempts struct {
	mu      sync.Mutex
	entries map[string]*loginAttempt
}

type loginAttempt struct {
	failures int
	blocked  time.Time
}

var loginLimiter = &loginAttempts{entries: map[string]*loginAttempt{}}

func backoffFor(failures int) time.Duration {
	if failures <= freeLoginAttempts {
		return 0
	}
	backoff := time.Second << (failures - freeLoginAttempts - 1)
	if backoff > maxLoginBackoff || backoff <= 0 {
		return maxLoginBackoff
	}
	return backoff
}

func (l *loginAttempts) allow(keys ...string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)

	for _, key := range keys {
		if entry, ok := l.entries[key]; ok && now.Before(entry.blocked) {
			return false
		}
	}
	return true
}

func (l *loginAttempts) fail(keys ...string) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)

	for _, key := range keys {
		entry, ok := l.entries[key]
		if !ok {
			entry = &loginAttempt{}
			l.entries[key] = entry
		}
		entry.failures++
		if backoff := backoffFor(entry.failures); backoff > 0 {
			entry.blocked = now.Add(backoff)
		}
	}
}

func (l *loginAttempts) succeed(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range keys {
		delete(l.entries, key)
	}
}

func (l *loginAttempts) prune(now time.Time) {
	for key, entry := range l.entries {
		if entry.blocked.IsZero() || now.After(entry.blocked.Add(maxLoginBackoff)) {
			if entry.failures <= freeLoginAttempts && entry.blocked.IsZero() {
				continue
			}
			delete(l.entries, key)
		}
	}
}

// RemoteAddr only: X-Forwarded-For is caller-controlled and would let an
// attacker rotate past the limit.
func clientAddr(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}
