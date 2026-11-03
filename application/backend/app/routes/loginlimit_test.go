package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
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
		req := jsonRequest("POST", "/api/v1/login", body)
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
		req := jsonRequest("POST", "/api/v1/login", body)
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

func TestInFlightLoginAttemptsCountAgainstTheLimit(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	keys := []string{"ip:203.0.113.9", "pair:" + loginKey("203.0.113.9", "someone")}

	admitted := 0
	for i := 0; i < 40; i++ {
		if limiter.allow(keys...) {
			admitted++
		}
	}
	if admitted != freeLoginAttempts+1 {
		t.Fatalf("%d unsettled attempts were let through at once, serially only %d would be", admitted, freeLoginAttempts+1)
	}

	for i := 0; i < admitted; i++ {
		limiter.fail(keys...)
	}
	if limiter.allow(keys...) {
		t.Fatal("not blocked after the burst failed")
	}

	limiter.mu.Lock()
	for _, key := range keys {
		limiter.entries[key].blocked = time.Now().Add(-time.Second)
	}
	limiter.mu.Unlock()
	if !limiter.allow(keys...) {
		t.Fatal("still blocked after the backoff expired")
	}
	if limiter.allow(keys...) {
		t.Fatal("a second attempt ran alongside the one the expired block allows")
	}
	limiter.succeed(keys[0], keys[1])
	if !limiter.allow(keys...) {
		t.Fatal("a successful login did not clear the limit")
	}
}

func TestConcurrentWrongLoginsGetNoMoreChecksThanSerialOnes(t *testing.T) {
	userdb.CreateUser("burstvictim", "the-real-password")
	t.Cleanup(func() { userdb.DeleteUser("burstvictim") })

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	var reached atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{"Login": "burstvictim", "Password": "wrong"})
			req := jsonRequest("POST", "/api/v1/login", body)
			req.RemoteAddr = "198.51.100.7:40000"
			rr := httptest.NewRecorder()
			<-start
			http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
			if rr.Code != http.StatusTooManyRequests {
				reached.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := reached.Load(); got > freeLoginAttempts+1 {
		t.Fatalf("%d concurrent wrong passwords were checked, serially only %d would be", got, freeLoginAttempts+1)
	}
}

// Simulates d passing for every entry.
func ageLoginAttempts(l *loginAttempts, d time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, entry := range l.entries {
		entry.seen = entry.seen.Add(-d)
		entry.blocked = entry.blocked.Add(-d)
	}
}

func TestLoggingIntoYourOwnAccountDoesNotResetTheAddressLimit(t *testing.T) {
	userdb.CreateUser("sprayowner", "own-password")
	t.Cleanup(func() { userdb.DeleteUser("sprayowner") })

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	attempt := func(login, password string) int {
		body, _ := json.Marshal(map[string]string{"Login": login, "Password": password})
		req := jsonRequest("POST", "/api/v1/login", body)
		req.RemoteAddr = "203.0.113.20:40000"
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Code
	}

	// Guesses at other accounts, each round followed by a login to one the
	// attacker holds.
	guessed := 0
	for round := 0; round < 4; round++ {
		for i := 0; i < freeLoginAttempts; i++ {
			if attempt("target-"+strconv.Itoa(round*freeLoginAttempts+i), "guess") != http.StatusTooManyRequests {
				guessed++
			}
		}
		attempt("sprayowner", "own-password")
	}
	if guessed > freeLoginAttempts+1 {
		t.Fatalf("%d guesses got through from one address; without the logins in between only %d would", guessed, freeLoginAttempts+1)
	}
}

func TestTyposBeforeALoginDoNotPileUpAcrossVisits(t *testing.T) {
	userdb.CreateUser("typouser", "the-real-password")
	t.Cleanup(func() { userdb.DeleteUser("typouser") })

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	attempt := func(password string) (int, bool) {
		body, _ := json.Marshal(map[string]string{"Login": "typouser", "Password": password})
		req := jsonRequest("POST", "/api/v1/login", body)
		req.RemoteAddr = "198.51.100.20:40000"
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Code, len(rr.Result().Cookies()) > 0
	}

	// Each visit stays inside the free allowance and starts more than an hour
	// after the last typo, though a login in between keeps the address busy.
	for visit := 1; visit <= 3; visit++ {
		loggedIn := func(when string) {
			t.Helper()
			if code, ok := attempt("the-real-password"); !ok {
				t.Fatalf("visit %d, %s: the correct password was refused: status %d", visit, when, code)
			}
		}
		for i := 0; i < freeLoginAttempts-2; i++ {
			attempt("typo-" + strconv.Itoa(i))
		}
		loggedIn("after the typos")
		loggedIn("logging in again straight away")
		ageLoginAttempts(loginLimiter, 40*time.Minute)
		loggedIn("a later login from the same address")
		ageLoginAttempts(loginLimiter, 40*time.Minute)
	}
}

func TestAnOfficeBehindOneAddressIsNotLockedOutByItsTypos(t *testing.T) {
	users := []string{"officealice", "officebob", "officecarol", "officeboss"}
	for _, user := range users {
		userdb.CreateUser(user, user+"-password")
	}
	t.Cleanup(func() {
		for _, user := range users {
			userdb.DeleteUser(user)
		}
	})

	loginLimiter.mu.Lock()
	loginLimiter.entries = map[string]*loginAttempt{}
	loginLimiter.mu.Unlock()

	attempt := func(login, password string) (int, bool) {
		body, _ := json.Marshal(map[string]string{"Login": login, "Password": password})
		req := jsonRequest("POST", "/api/v1/login", body)
		req.RemoteAddr = "198.51.100.50:40000"
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Code, len(rr.Result().Cookies()) > 0
	}

	// Every ten minutes someone mistypes once and then gets it right; the boss
	// never mistypes.
	for event := 1; event <= 18; event++ {
		user := users[event%3]
		if code, _ := attempt(user, "typo"); code == http.StatusTooManyRequests {
			t.Fatalf("event %d: %s's typo was refused outright", event, user)
		}
		if code, ok := attempt(user, user+"-password"); !ok {
			t.Fatalf("event %d: %s was refused after one typo: status %d", event, user, code)
		}
		if code, ok := attempt("officeboss", "officeboss-password"); !ok {
			t.Fatalf("event %d: a user who never mistyped was refused: status %d", event, code)
		}
		ageLoginAttempts(loginLimiter, 10*time.Minute)
	}
}

func TestSharedAddressTyposDoNotPileUp(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	addr := "192.0.2.50"
	ip := "ip:" + addr

	// Each user mistypes and logs in 20 minutes later; the next typo lands 50
	// minutes after the last, so the address is never quiet for an hour.
	for i := 0; i < 17; i++ {
		pair := "pair:" + loginKey(addr, "user"+strconv.Itoa(i))
		if !limiter.allow(ip, pair) {
			t.Fatalf("round %d: the typo was refused", i)
		}
		limiter.fail(ip, pair)
		ageLoginAttempts(limiter, 20*time.Minute)
		if !limiter.allow(ip, pair) {
			t.Fatalf("round %d: the correct retry was refused", i)
		}
		limiter.succeed(ip, pair)
		ageLoginAttempts(limiter, 30*time.Minute)
	}

	typo := "pair:" + loginKey(addr, "late")
	limiter.allow(ip, typo)
	limiter.fail(ip, typo)
	if !limiter.allow(ip, "pair:"+loginKey(addr, "boss")) {
		t.Fatalf("a user with the right password was refused: the address holds %d failures", limiter.entries[ip].failures)
	}
}

func TestOverlappingLoginsAfterForgivenTyposGetTheCleanAllowance(t *testing.T) {
	addr := "198.51.100.60"
	ip := "ip:" + addr
	carol := "pair:" + loginKey(addr, "carol")
	users := []string{"alice", "bob", "carol"}

	for _, typos := range []int{0, 3, freeLoginAttempts} {
		limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
		for i := 0; i < typos; i++ {
			limiter.allow(ip, carol)
			limiter.fail(ip, carol)
		}
		if !limiter.allow(ip, carol) {
			t.Fatalf("%d typos: carol's correct password was refused", typos)
		}
		limiter.succeed(ip, carol)

		admitted := 0
		for i := 0; i < 10; i++ {
			if limiter.allow(ip, "pair:"+loginKey(addr, users[i%3])) {
				admitted++
			}
		}
		if admitted != freeLoginAttempts+1 {
			t.Errorf("after %d typos and a login, %d of 10 overlapping logins got through; from a clean address %d do",
				typos, admitted, freeLoginAttempts+1)
		}
	}
}

func TestALoginForgivesNoMoreThanTheAddressHolds(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	addr := "203.0.113.30"
	ip, own := "ip:"+addr, "pair:"+loginKey(addr, "own")
	// The address entry has faded further than the account's.
	limiter.entries[ip] = &loginAttempt{failures: 2, seen: time.Now()}
	limiter.entries[own] = &loginAttempt{failures: freeLoginAttempts, seen: time.Now()}
	limiter.allow(ip, own)
	limiter.succeed(ip, own)

	guessed := 0
	for i := 0; i < 20; i++ {
		target := "pair:" + loginKey(addr, "target-"+strconv.Itoa(i))
		if limiter.allow(ip, target) {
			limiter.fail(ip, target)
			guessed++
		}
	}
	if guessed > freeLoginAttempts+1 {
		t.Fatalf("%d guesses got through after the login; from a clean address only %d would", guessed, freeLoginAttempts+1)
	}
}

func TestForgivenTyposLiftTheAddressBlock(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	addr := "198.51.100.80"
	ip := "ip:" + addr
	alice, bob := "pair:"+loginKey(addr, "alice"), "pair:"+loginKey(addr, "bob")
	limiter.fail(ip, alice)
	limiter.fail(ip, alice)
	for i := 0; i < freeLoginAttempts-1; i++ {
		limiter.fail(ip, bob)
	}
	limiter.succeed(ip, alice)

	if !limiter.allow(ip, "pair:"+loginKey(addr, "carol")) {
		t.Fatal("the address stayed blocked though alice's typos were forgiven")
	}
}

func TestPausingAnHourDoesNotHandBackTheWholeAllowance(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	keys := []string{"ip:203.0.113.40", "pair:" + loginKey("203.0.113.40", "admin")}
	for i := 0; i < 15; i++ {
		limiter.fail(keys...)
	}
	ageLoginAttempts(limiter, time.Hour+time.Minute)

	guessed := 0
	for limiter.allow(keys...) {
		limiter.fail(keys...)
		guessed++
	}
	if guessed > 1 {
		t.Fatalf("after 15 failures and an hour's pause, %d guesses got through back to back", guessed)
	}
}

func TestFailuresLeakOnePerInterval(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	keys := []string{"ip:198.51.100.70", "pair:" + loginKey("198.51.100.70", "someone")}
	for i := 0; i < 3; i++ {
		limiter.fail(keys...)
	}

	// Someone else's failures prune.
	ageLoginAttempts(limiter, leakInterval+time.Minute)
	limiter.fail("ip:192.0.2.99")
	for _, key := range keys {
		if entry := limiter.entries[key]; entry == nil || entry.failures != 2 {
			t.Fatalf("%s: one interval on, three failures should have leaked to two, got %+v", key, entry)
		}
	}

	ageLoginAttempts(limiter, 2*leakInterval)
	limiter.fail("ip:192.0.2.99")
	for _, key := range keys {
		if entry, ok := limiter.entries[key]; ok {
			t.Errorf("%s still holds %d failures three intervals on", key, entry.failures)
		}
	}
}

// A client with a stale password fails every 20 minutes all day behind an
// office's address, and is fixed in the evening.
func TestAStaleClientDoesNotLockOutItsAddress(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	addr := "198.51.100.90"
	ip := "ip:" + addr
	stale := "pair:" + loginKey(addr, "stale-script")
	colleague := "pair:" + loginKey(addr, "colleague")
	for round := 1; round <= 24; round++ {
		if limiter.allow(ip, stale) {
			limiter.fail(ip, stale)
		}
		ageLoginAttempts(limiter, time.Minute)
		if !limiter.allow(ip, colleague) {
			t.Fatalf("round %d: a colleague logging in a minute after the stale client failed was refused", round)
		}
		limiter.succeed(ip, colleague)
		ageLoginAttempts(limiter, 19*time.Minute)
	}

	ageLoginAttempts(limiter, 16*time.Hour)
	typist := "pair:" + loginKey(addr, "typist")
	limiter.allow(ip, typist)
	limiter.fail(ip, typist)
	if !limiter.allow(ip, typist) {
		t.Fatal("the next morning, one typo held up the correct password")
	}
	limiter.succeed(ip, typist)
	admitted := 0
	for i := 0; i < 10; i++ {
		if limiter.allow(ip, "pair:"+loginKey(addr, "office-"+strconv.Itoa(i))) {
			admitted++
		}
	}
	if admitted != freeLoginAttempts+1 {
		t.Fatalf("the next morning %d of 10 overlapping logins got through; from a clean address %d do", admitted, freeLoginAttempts+1)
	}
}

// Every 30 minutes someone mistypes once and retypes; every hour someone else
// mistypes three times and logs in only after lunch, 90 minutes later.
func TestTyposForgivenHoursLaterDoNotBuildUp(t *testing.T) {
	limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
	addr := "198.51.100.95"
	ip := "ip:" + addr
	late := map[int]string{}
	for event := 1; event <= 16; event++ {
		typist := "pair:" + loginKey(addr, "typist-"+strconv.Itoa(event%4))
		if limiter.allow(ip, typist) {
			limiter.fail(ip, typist)
		}
		ageLoginAttempts(limiter, 5*time.Second)
		if !limiter.allow(ip, typist) {
			t.Fatalf("event %d: the correct retry five seconds after one typo was refused", event)
		}
		limiter.succeed(ip, typist)

		if event%2 == 1 {
			late[event+3] = "pair:" + loginKey(addr, "late-"+strconv.Itoa(event))
			for i := 0; i < 3; i++ {
				if limiter.allow(ip, late[event+3]) {
					limiter.fail(ip, late[event+3])
				}
			}
		}
		if pair, ok := late[event]; ok {
			if !limiter.allow(ip, pair) {
				t.Fatalf("event %d: a user back from lunch was refused", event)
			}
			limiter.succeed(ip, pair)
		}
		ageLoginAttempts(limiter, 30*time.Minute)
	}
}

// One address guessing at one account for a day, greedily or in bursts with
// an hour's pause after each.
func TestALoneAttackerGetsOneGuessPerLeakInterval(t *testing.T) {
	for _, burst := range []int{0, 6, 10, 15} {
		limiter := &loginAttempts{entries: map[string]*loginAttempt{}}
		keys := []string{"ip:203.0.113.50", "pair:" + loginKey("203.0.113.50", "admin")}
		guesses, inBurst, idle := 0, 0, time.Duration(0)
		for elapsed := time.Duration(0); elapsed < 24*time.Hour; elapsed += time.Second {
			if idle > 0 {
				idle -= time.Second
			} else {
				for limiter.allow(keys...) {
					limiter.fail(keys...)
					guesses++
					if inBurst++; inBurst == burst {
						inBurst, idle = 0, time.Hour+time.Minute
						break
					}
				}
			}
			ageLoginAttempts(limiter, time.Second)
		}
		// 16 guesses reach the 15-minute block; after that, one per leak.
		if limit := 16 + int(24*time.Hour/leakInterval) - 1; guesses > limit {
			t.Errorf("bursts of %d: %d guesses in a day, more than %d", burst, guesses, limit)
		}
	}
}
