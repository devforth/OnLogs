package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
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

// Empty means no peer may speak for anyone else.
var trustedProxies []netip.Prefix

// SetTrustedProxies takes a comma separated list of IPs and CIDR ranges.
func SetTrustedProxies(spec string) error {
	parsed, err := parseTrustedProxies(spec)
	if err != nil {
		return err
	}
	trustedProxies = parsed
	return nil
}

func parseTrustedProxies(spec string) ([]netip.Prefix, error) {
	var parsed []netip.Prefix
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		switch {
		case item == "":
		case strings.Contains(item, "/"):
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, fmt.Errorf("%q is not a CIDR range: %w", item, err)
			}
			parsed = append(parsed, prefix.Masked())
		default:
			addr, err := netip.ParseAddr(item)
			if err != nil {
				return nil, fmt.Errorf("%q is not an IP address or CIDR range: %w", item, err)
			}
			addr = addr.Unmap()
			parsed = append(parsed, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return parsed, nil
}

func isTrustedProxy(addr netip.Addr) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// Zones and the IPv4-in-IPv6 form are dropped so one client cannot hold two
// buckets.
func parseIP(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap().WithZone(""), true
	}
	if addrPort, err := netip.ParseAddrPort(value); err == nil {
		return addrPort.Addr().Unmap().WithZone(""), true
	}
	return netip.Addr{}, false
}

// The peer address, unless it is a trusted proxy: then the address it
// forwarded. The headers are caller-controlled, so taking them from anyone
// would let an attacker rotate past the limit or wear someone else's address.
func clientAddr(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}

	peer, ok := parseIP(host)
	if !ok {
		return host
	}
	if !isTrustedProxy(peer) {
		warnUntrustedForward(req)
		return peer.String()
	}
	if forwarded, ok := forwardedClient(req); ok {
		return forwarded
	}
	return peer.String()
}

// Right to left: every hop appends the address it saw, so the rightmost entry
// no trusted proxy could have written is the last one still worth believing.
func forwardedClient(req *http.Request) (string, bool) {
	entries := strings.Split(strings.Join(req.Header.Values("X-Forwarded-For"), ","), ",")
	origin := ""
	for i := len(entries) - 1; i >= 0; i-- {
		addr, ok := parseIP(entries[i])
		if !ok {
			break
		}
		if !isTrustedProxy(addr) {
			return addr.String(), true
		}
		origin = addr.String()
	}

	if addr, ok := parseIP(req.Header.Get("X-Real-IP")); ok {
		return addr.String(), true
	}
	// Every hop was trusted, so the chain names its own origin.
	return origin, origin != ""
}

var forwardWarning sync.Once

// Either a proxy nobody told us about, whose users then share one bucket, or
// someone trying on another address for size.
func warnUntrustedForward(req *http.Request) {
	if req.Header.Get("X-Forwarded-For") == "" && req.Header.Get("X-Real-IP") == "" {
		return
	}
	forwardWarning.Do(func() {
		fmt.Printf("WARNING: a login from %s carried a forwarded client address, but that peer is not in TRUSTED_PROXIES;"+
			" rate limiting keys on the peer, so everyone behind it shares one limit.\n", req.RemoteAddr)
	})
}
