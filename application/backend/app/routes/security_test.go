package routes

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestFrontendDoesNotServeFilesOutsideDist(t *testing.T) {
	os.MkdirAll("dist", 0o700)
	os.WriteFile("dist/index.html", []byte("text"), 0o600)

	os.MkdirAll("leveldb_probe", 0o700)
	os.WriteFile("leveldb_probe/JWT_secret", []byte("REAL-ONLOGS-JWT-SECRET"), 0o600)
	t.Cleanup(func() { os.RemoveAll("leveldb_probe") })

	for _, target := range []string{
		"/?x=/../../leveldb_probe/JWT_secret",
		"/index.html?x=/../../leveldb_probe/JWT_secret",
		"/?/../../leveldb_probe/JWT_secret",
	} {
		req, _ := http.NewRequest("GET", target, nil)
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Frontend).ServeHTTP(rr, req)
		body, _ := io.ReadAll(rr.Result().Body)
		if strings.Contains(string(body), "REAL-ONLOGS-JWT-SECRET") {
			t.Fatalf("arbitrary file read through %q: %s", target, string(body))
		}
	}
}

func TestFrontendStripsPathPrefixOnlyAtTheFront(t *testing.T) {
	t.Setenv("ONLOGS_PATH_PREFIX", "/logs")

	os.MkdirAll("dist/assets", 0o700)
	os.WriteFile("dist/index.html", []byte("text"), 0o600)
	os.WriteFile("dist/assets/logs-panel.js", []byte("PANEL_ASSET"), 0o600)
	t.Cleanup(func() { os.RemoveAll("dist/assets") })

	req, _ := http.NewRequest("GET", "/logs/assets/logs-panel.js", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.Frontend).ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Result().Body)
	if string(body) != "PANEL_ASSET" {
		t.Fatalf("expected the asset, got %q", string(body))
	}
}

func adminOnlyRequest(t *testing.T, handler http.HandlerFunc, method, target string) int {
	t.Helper()
	os.Setenv("ADMIN_USERNAME", "admin")

	req, _ := http.NewRequest(method, target, nil)
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("viewer")})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr.Result().StatusCode
}

func TestGetSecretIsAdminOnly(t *testing.T) {
	if code := adminOnlyRequest(t, testCtrl.GetSecret, "GET", "/api/v1/getSecret"); code != http.StatusForbidden {
		t.Fatalf("a non-admin account minted an agent token: status %d", code)
	}
}

func TestGetUsersIsAdminOnly(t *testing.T) {
	if code := adminOnlyRequest(t, testCtrl.GetUsers, "GET", "/api/v1/getUsers"); code != http.StatusForbidden {
		t.Fatalf("a non-admin account enumerated every username: status %d", code)
	}
}

func TestAddHostRejectsNamesThatLeaveTheTree(t *testing.T) {
	token := db.CreateOnLogsToken()

	for _, payload := range []map[string]interface{}{
		{"Hostname": "../../../../PWNED_HOST", "Token": token, "Services": []string{"c"}},
		{"Hostname": "realhost", "Token": token, "Services": []string{"../../../../PWNED_SERVICE"}},
	} {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/addHost", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.AddHost).ServeHTTP(rr, req)

		if code := rr.Result().StatusCode; code != http.StatusBadRequest {
			t.Errorf("addHost accepted %v: status %d", payload["Hostname"], code)
		}
	}

	// Assert on exactly the paths MkdirAll would build, not on names relative to
	// the package directory — those resolve elsewhere and silently pass.
	for _, leaked := range []string{
		"leveldb/hosts/../../../../PWNED_HOST",
		"leveldb/hosts/realhost/containers/../../../../PWNED_SERVICE",
	} {
		if _, err := os.Stat(leaked); err == nil {
			os.RemoveAll(leaked)
			t.Errorf("addHost created %s outside leveldb/hosts", leaked)
		}
	}
	entries, _ := os.ReadDir("leveldb/hosts")
	for _, entry := range entries {
		if !util.IsSafeName(entry.Name()) {
			t.Errorf("addHost created an unsafe host directory: %q", entry.Name())
		}
	}
}

func TestAddLogLineRejectsNamesThatLeaveTheTree(t *testing.T) {
	token := db.CreateOnLogsToken()

	body, _ := json.Marshal(map[string]interface{}{
		"Token":     token,
		"Host":      "../../../../PWNED_INGEST",
		"Container": "c",
		"LogLine":   []string{"2026-02-10T12:56:09.230421754Z", "hello"},
	})
	req, _ := http.NewRequest("POST", "/api/v1/addLogLine", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.AddLogLine).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusBadRequest {
		t.Errorf("addLogLine accepted a traversing host: status %d", code)
	}
	for _, leaked := range []string{"../../../../PWNED_INGEST", "leveldb/hosts/../../../../PWNED_INGEST"} {
		if _, err := os.Stat(leaked); err == nil {
			os.RemoveAll(leaked)
			t.Errorf("addLogLine created %s outside leveldb/hosts", leaked)
		}
	}
}

func oversizedJSONBody() []byte {
	body := []byte(`{"pad":"`)
	body = append(body, bytes.Repeat([]byte("A"), 4<<20)...)
	return append(body, []byte(`"}`)...)
}

func TestHandlersRejectOversizedRequestBodies(t *testing.T) {
	os.Setenv("ADMIN_USERNAME", "admin")

	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"updateUserSettings", testCtrl.UpdateUserSettings},
		{"login", testCtrl.Login},
		{"changeFavorite", testCtrl.ChangeFavourite},
		{"addLogLine", testCtrl.AddLogLine},
	}

	for _, c := range cases {
		req, _ := http.NewRequest("POST", "/", bytes.NewBuffer(oversizedJSONBody()))
		req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})
		rr := httptest.NewRecorder()
		c.handler.ServeHTTP(rr, req)

		if code := rr.Result().StatusCode; code == http.StatusOK {
			t.Errorf("%s accepted a 4 MiB request body: status %d", c.name, code)
		}
	}
}

func TestLoginCookieIsHttpOnlyAndUsesARelativeMaxAge(t *testing.T) {
	userdb.CreateUser("cookieuser", "cookiepass")

	body, _ := json.Marshal(map[string]string{"Login": "cookieuser", "Password": "cookiepass"})
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login set no cookie: %s", rr.Body.String())
	}
	c := cookies[0]

	if !c.HttpOnly {
		t.Error("session cookie is readable from JavaScript, so any XSS lifts the session")
	}
	if c.MaxAge > 7*24*3600 {
		t.Errorf("Max-Age is %d seconds (~%d years); it should be a relative lifetime, not an absolute epoch",
			c.MaxAge, c.MaxAge/(365*24*3600))
	}
	if c.MaxAge <= 0 {
		t.Errorf("Max-Age is %d, the session would not persist", c.MaxAge)
	}
}

func TestLoginCookieIsSecureOverTLS(t *testing.T) {
	userdb.CreateUser("cookieuser", "cookiepass")

	body, _ := json.Marshal(map[string]string{"Login": "cookieuser", "Password": "cookiepass"})
	req, _ := http.NewRequest("POST", "https://onlogs.example/api/v1/login", bytes.NewBuffer(body))
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login set no cookie: %s", rr.Body.String())
	}
	if !cookies[0].Secure {
		t.Error("session cookie issued over TLS is not marked Secure")
	}
}

func TestGetLogsStreamRejectsAForeignOrigin(t *testing.T) {

	req, _ := http.NewRequest("GET", "/api/v1/getLogsStream?host="+util.GetHost()+"&id=somecontainer", nil)
	req.Host = "onlogs.example"
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})

	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.GetLogsStream).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusForbidden {
		t.Errorf("a cross-origin websocket handshake was not rejected: status %d", code)
	}
	if got := vars.ConnectionCount(util.GetHost() + "/somecontainer"); got != 0 {
		t.Fatalf("a failed upgrade stored %d connection(s); a background goroutine will dereference them", got)
	}
}

func TestLoginRateLimitsRepeatedFailures(t *testing.T) {
	userdb.CreateUser("ratelimited", "correct-horse")

	post := func(password string) int {
		body, _ := json.Marshal(map[string]string{"Login": "ratelimited", "Password": password})
		req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
		req.RemoteAddr = "203.0.113.9:34567"
		rr := httptest.NewRecorder()
		http.HandlerFunc(testCtrl.Login).ServeHTTP(rr, req)
		return rr.Result().StatusCode
	}

	for i := 0; i < 20; i++ {
		post("wrong-" + strconv.Itoa(i))
	}

	if code := post("correct-horse"); code != http.StatusTooManyRequests {
		t.Fatalf("20 failed logins from one address did not trigger a backoff: status %d", code)
	}
}

func TestAddLogLineDoesNotSpawnAWorkerPerLogLine(t *testing.T) {
	token := db.CreateOnLogsToken()
	before := statistics.WorkerCount()
	t.Cleanup(func() { statistics.StopWorker("statsprobehost", "statsprobecontainer") })

	ingest := func(count int) {
		for i := 0; i < count; i++ {
			body, _ := json.Marshal(map[string]interface{}{
				"Token":     token,
				"Host":      "statsprobehost",
				"Container": "statsprobecontainer",
				"LogLine":   []string{"2026-02-10T12:56:09.230421754Z", "line " + strconv.Itoa(i)},
			})
			req, _ := http.NewRequest("POST", "/api/v1/addLogLine", bytes.NewBuffer(body))
			rr := httptest.NewRecorder()
			http.HandlerFunc(testCtrl.AddLogLine).ServeHTTP(rr, req)
			if code := rr.Result().StatusCode; code != http.StatusOK {
				t.Fatalf("ingestion failed: status %d", code)
			}
		}
		time.Sleep(250 * time.Millisecond)
	}

	// Warm up: the first line legitimately starts one worker, which opens
	// databases and so brings its own goroutines with it.
	ingest(25)
	settled := runtime.NumGoroutine()

	ingest(25)
	growth := runtime.NumGoroutine() - settled

	if got := statistics.WorkerCount() - before; got != 1 {
		t.Errorf("expected exactly one registered statistics worker, got %d", got)
	}
	if growth > 5 {
		t.Fatalf("a further 25 log lines started %d more goroutines; the leak scales with ingestion and each worker zeroes the live counter", growth)
	}
}

// The Content-Type was previously set per handler; a helper now owns it, so one
// endpoint of each shape has to prove the header survives.
func TestJSONEndpointsSetTheirContentType(t *testing.T) {
	os.Setenv("ADMIN_USERNAME", "admin")

	cases := []struct {
		name    string
		handler http.HandlerFunc
		user    string
	}{
		{"checkCookie", testCtrl.CheckCookie, "testuser"},
		{"getHosts", testCtrl.GetHosts, "testuser"},
		{"getUsers", testCtrl.GetUsers, "admin"},
		{"getSizeByAll", testCtrl.GetSizeByAll, "testuser"},
		{"getGroups", testCtrl.GetGroups, "testuser"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/", nil)
			req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT(c.user)})
			rr := httptest.NewRecorder()
			c.handler.ServeHTTP(rr, req)

			if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
				t.Errorf("%s replied with Content-Type %q, want application/json", c.name, got)
			}
		})
	}
}

func TestRejectedRequestsStillSetContentType(t *testing.T) {

	req, _ := http.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	testCtrl.CheckCookie(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("a 401 replied with Content-Type %q, want application/json", got)
	}
}
