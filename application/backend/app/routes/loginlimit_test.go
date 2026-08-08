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

// The lockout lived in which key the handler chose, so it has to be exercised
// through the handler.
func TestLoginLockoutCannotBeInflictedOnAnotherUser(t *testing.T) {
	ctrl := initTestConfig()
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
		http.HandlerFunc(ctrl.Login).ServeHTTP(rr, req)
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
