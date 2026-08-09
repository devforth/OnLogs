package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/userdb"
)

func TestLoginLimiterCannotBeUsedToLockOutAnAccount(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}

	attacker := "203.0.113.9"
	victim := "198.51.100.7"

	// The attacker guesses at the admin account from their own address.
	for i := 0; i < 50; i++ {
		limiter.fail("ip:"+attacker, "pair:"+loginKey(attacker, "admin"))
	}

	if limiter.allow("ip:" + attacker) {
		t.Error("the attacker was not throttled")
	}
	if !limiter.allow("ip:"+victim, "pair:"+loginKey(victim, "admin")) {
		t.Fatal("an attacker guessing at an account locked the real owner out of it")
	}
}

func TestLoginLimiterDoesNotGrowWithoutBound(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}

	// One failed attempt per distinct username, as an unauthenticated attacker
	// can produce at will.
	for i := 0; i < maxLoginEntries*3; i++ {
		name := "user-" + strconv.Itoa(i)
		limiter.fail("pair:" + loginKey("203.0.113.9", name))
	}

	limiter.mu.Lock()
	size := len(limiter.entries)
	limiter.mu.Unlock()

	if size > maxLoginEntries {
		t.Fatalf("the limiter retained %d entries for %d attempts; memory grows without bound",
			size, maxLoginEntries*3)
	}
}

func TestLoginLimiterKeysAreBounded(t *testing.T) {
	huge := make([]byte, 900*1024)
	for i := range huge {
		huge[i] = 'a'
	}
	if got := len(loginKey("203.0.113.9", string(huge))); got > 64 {
		t.Fatalf("a %d-byte login produced a %d-byte key", len(huge), got)
	}
}

func TestLoginLimiterStillThrottlesRepeatedFailures(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	key := "pair:" + loginKey("203.0.113.9", "someone")

	for i := 0; i < freeLoginAttempts; i++ {
		if !limiter.allow(key) {
			t.Fatalf("throttled after only %d attempts", i)
		}
		limiter.fail(key)
	}
	limiter.fail(key)

	if limiter.allow(key) {
		t.Fatal("repeated failures were not throttled")
	}
	if backoffFor(100) != maxLoginBackoff {
		t.Errorf("backoff did not clamp: %v", backoffFor(100))
	}
	if backoffFor(freeLoginAttempts) != 0 {
		t.Errorf("throttled inside the free allowance: %v", backoffFor(freeLoginAttempts))
	}
	_ = time.Second
}

func withTrustedProxies(t *testing.T, spec string) {
	t.Helper()
	if err := SetTrustedProxies(spec); err != nil {
		t.Fatalf("SetTrustedProxies(%q): %v", spec, err)
	}
	t.Cleanup(func() { trustedProxies = nil })
}

func forwardedRequest(peer string, headers map[string]string) *http.Request {
	req, _ := http.NewRequest("POST", "/api/v1/login", nil)
	req.RemoteAddr = peer
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return req
}

func TestForwardedAddressesAreIgnoredFromUntrustedPeers(t *testing.T) {
	req := forwardedRequest("203.0.113.9:40000", map[string]string{
		"X-Forwarded-For": "198.51.100.7",
		"X-Real-IP":       "198.51.100.7",
	})

	if got := clientAddr(req); got != "203.0.113.9" {
		t.Fatalf("a caller wore another address with no proxy configured: %q", got)
	}

	// Trusting some other proxy is not trusting this caller.
	withTrustedProxies(t, "192.0.2.1,10.0.0.0/8")
	if got := clientAddr(req); got != "203.0.113.9" {
		t.Fatalf("a caller outside TRUSTED_PROXIES wore another address: %q", got)
	}
}

func TestClientAddrResolvesThroughTrustedProxies(t *testing.T) {
	withTrustedProxies(t, "10.0.0.0/8,172.16.0.0/12")

	cases := []struct {
		name    string
		peer    string
		headers map[string]string
		want    string
	}{
		{"single hop", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "198.51.100.7"}, "198.51.100.7"},
		{"chained proxies", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "198.51.100.7, 10.0.0.5"}, "198.51.100.7"},
		{"client prepended a fake hop", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "1.1.1.1, 198.51.100.7"}, "198.51.100.7"},
		{"x-real-ip only", "10.0.0.1:40000", map[string]string{"X-Real-IP": "198.51.100.7"}, "198.51.100.7"},
		{"entry carries a port", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "198.51.100.7:1234"}, "198.51.100.7"},
		{"ipv4 in ipv6 form", "[::ffff:10.0.0.1]:40000", map[string]string{"X-Forwarded-For": "::ffff:198.51.100.7"}, "198.51.100.7"},
		{"unparseable chain", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "not-an-address"}, "10.0.0.1"},
		{"proxy forwarded nothing", "10.0.0.1:40000", nil, "10.0.0.1"},
		{"internal end to end", "10.0.0.1:40000", map[string]string{"X-Forwarded-For": "10.9.9.9, 10.0.0.5"}, "10.9.9.9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientAddr(forwardedRequest(tc.peer, tc.headers)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTrustedProxySpecIsValidated(t *testing.T) {
	if _, err := parseTrustedProxies(" 10.0.0.0/8 , 192.0.2.1,::1 "); err != nil {
		t.Fatalf("a valid spec was rejected: %v", err)
	}
	if parsed, err := parseTrustedProxies(""); err != nil || parsed != nil {
		t.Fatalf("an empty spec gave (%v, %v)", parsed, err)
	}
	for _, spec := range []string{"10.0.0.0/99", "not-an-address", "10.0.0.1-10.0.0.5", "10.0.0.0/8,junk"} {
		if _, err := parseTrustedProxies(spec); err == nil {
			t.Errorf("%q was accepted", spec)
		}
	}
}

// The lockout lived in which key the handler chose, so it has to be exercised
// through the handler.
func TestLoginLockoutCannotBeInflictedOnAnotherUser(t *testing.T) {
	userdb.CreateUser("victimaccount", "the-real-password")
	t.Cleanup(func() { userdb.DeleteUser("victimaccount") })

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	attempt := func(addr, password string) int {
		body, _ := json.Marshal(map[string]string{"Login": "victimaccount", "Password": password})
		req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
		req.RemoteAddr = addr + ":40000"
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Result().StatusCode
	}

	// An attacker guesses at the account from their own address.
	for i := 0; i < 20; i++ {
		attempt("203.0.113.9", "guess-"+strconv.Itoa(i))
	}
	if code := attempt("203.0.113.9", "the-real-password"); code != http.StatusTooManyRequests {
		t.Errorf("the attacker was not throttled: status %d", code)
	}

	if code := attempt("198.51.100.7", "the-real-password"); code != http.StatusOK {
		t.Fatalf("an attacker guessing at the account locked its real owner out: status %d", code)
	}
}

func TestOneAttackerBehindAProxyDoesNotThrottleEveryoneElse(t *testing.T) {
	userdb.CreateUser("proxieduser", "the-real-password")
	t.Cleanup(func() { userdb.DeleteUser("proxieduser") })
	withTrustedProxies(t, "10.0.0.0/8,172.16.0.0/12")

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	attempt := func(client, password string) int {
		body, _ := json.Marshal(map[string]string{"Login": "proxieduser", "Password": password})
		req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
		req.RemoteAddr = "172.18.0.2:40000"
		req.Header.Set("X-Forwarded-For", client)
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Result().StatusCode
	}

	for i := 0; i < 20; i++ {
		attempt("203.0.113.9", "guess-"+strconv.Itoa(i))
	}
	if code := attempt("203.0.113.9", "the-real-password"); code != http.StatusTooManyRequests {
		t.Errorf("the attacker was not throttled through the proxy: status %d", code)
	}
	if code := attempt("198.51.100.7", "the-real-password"); code != http.StatusOK {
		t.Fatalf("one attacker behind the proxy locked out everyone else: status %d", code)
	}
}
