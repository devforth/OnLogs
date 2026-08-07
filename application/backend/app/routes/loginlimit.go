package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	freeLoginAttempts = 5
	maxLoginBackoff   = 15 * time.Minute
	loginAttemptTTL   = time.Hour
	maxLoginEntries   = 4096
)

type loginAttempts struct {
	mu      sync.Mutex
	entries map[string]*loginAttempt
}

type loginAttempt struct {
	failures int
	blocked  time.Time
	seen     time.Time
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

// Keys are bounded in size: the login half is attacker-chosen and can be up to
// the whole request body.
func loginKey(addr string, login string) string {
	sum := sha256.Sum256([]byte(login))
	return addr + "|" + hex.EncodeToString(sum[:8])
}

func (l *loginAttempts) allow(keys ...string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

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
		entry.seen = now
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

// Called only from fail(), so the scan cost is bounded by the failure rate
// rather than paid on every login.
func (l *loginAttempts) prune(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.seen) > loginAttemptTTL && now.After(entry.blocked) {
			delete(l.entries, key)
		}
	}

	if len(l.entries) < maxLoginEntries {
		return
	}
	// Hard ceiling: drop everything that is not currently blocking anyone.
	for key, entry := range l.entries {
		if now.After(entry.blocked) {
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
