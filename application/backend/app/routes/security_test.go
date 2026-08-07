package routes

import (
	"crypto/tls"
	"strconv"

	"bytes"
	"encoding/json"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/vars"

	"github.com/devforth/OnLogs/app/db"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/util"
)

func TestFrontendDoesNotServeFilesOutsideDist(t *testing.T) {
	ctrl := initTestConfig()
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
		http.HandlerFunc(ctrl.Frontend).ServeHTTP(rr, req)
		body, _ := io.ReadAll(rr.Result().Body)
		if strings.Contains(string(body), "REAL-ONLOGS-JWT-SECRET") {
			t.Fatalf("arbitrary file read through %q: %s", target, string(body))
		}
	}
}

func TestFrontendStripsPathPrefixOnlyAtTheFront(t *testing.T) {
	ctrl := initTestConfig()
	t.Setenv("ONLOGS_PATH_PREFIX", "/logs")

	os.MkdirAll("dist/assets", 0o700)
	os.WriteFile("dist/index.html", []byte("text"), 0o600)
	os.WriteFile("dist/assets/logs-panel.js", []byte("PANEL_ASSET"), 0o600)
	t.Cleanup(func() { os.RemoveAll("dist/assets") })

	req, _ := http.NewRequest("GET", "/logs/assets/logs-panel.js", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.Frontend).ServeHTTP(rr, req)
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
	ctrl := initTestConfig()
	if code := adminOnlyRequest(t, ctrl.GetSecret, "GET", "/api/v1/getSecret"); code != http.StatusForbidden {
		t.Fatalf("a non-admin account minted an agent token: status %d", code)
	}
}

func TestGetUsersIsAdminOnly(t *testing.T) {
	ctrl := initTestConfig()
	if code := adminOnlyRequest(t, ctrl.GetUsers, "GET", "/api/v1/getUsers"); code != http.StatusForbidden {
		t.Fatalf("a non-admin account enumerated every username: status %d", code)
	}
}

func TestAddHostRejectsNamesThatLeaveTheTree(t *testing.T) {
	ctrl := initTestConfig()
	token := db.CreateOnLogsToken()

	for _, payload := range []map[string]interface{}{
		{"Hostname": "../../../../PWNED_HOST", "Token": token, "Services": []string{"c"}},
		{"Hostname": "realhost", "Token": token, "Services": []string{"../../../../PWNED_SERVICE"}},
	} {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/addHost", bytes.NewBuffer(body))
		rr := httptest.NewRecorder()
		http.HandlerFunc(ctrl.AddHost).ServeHTTP(rr, req)

		if code := rr.Result().StatusCode; code != http.StatusBadRequest {
			t.Errorf("addHost accepted %v: status %d", payload["Hostname"], code)
		}
	}

	for _, leaked := range []string{"PWNED_HOST", "PWNED_SERVICE", "../PWNED_HOST"} {
		if _, err := os.Stat(leaked); err == nil {
			os.RemoveAll(leaked)
			t.Errorf("addHost created %s outside leveldb/hosts", leaked)
		}
	}
}

func TestAddLogLineRejectsNamesThatLeaveTheTree(t *testing.T) {
	ctrl := initTestConfig()
	token := db.CreateOnLogsToken()

	body, _ := json.Marshal(map[string]interface{}{
		"Token":     token,
		"Host":      "../../../../PWNED_INGEST",
		"Container": "c",
		"LogLine":   []string{"2026-02-10T12:56:09.230421754Z", "hello"},
	})
	req, _ := http.NewRequest("POST", "/api/v1/addLogLine", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.AddLogLine).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusBadRequest {
		t.Errorf("addLogLine accepted a traversing host: status %d", code)
	}
	if _, err := os.Stat("../../../../PWNED_INGEST"); err == nil {
		os.RemoveAll("../../../../PWNED_INGEST")
		t.Error("addLogLine created a directory outside leveldb/hosts")
	}
}

func oversizedJSONBody() []byte {
	body := []byte(`{"pad":"`)
	body = append(body, bytes.Repeat([]byte("A"), 4<<20)...)
	return append(body, []byte(`"}`)...)
}

func TestHandlersRejectOversizedRequestBodies(t *testing.T) {
	ctrl := initTestConfig()
	os.Setenv("ADMIN_USERNAME", "admin")

	cases := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"updateUserSettings", ctrl.UpdateUserSettings},
		{"login", ctrl.Login},
		{"changeFavorite", ctrl.ChangeFavourite},
		{"addLogLine", ctrl.AddLogLine},
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
	ctrl := initTestConfig()
	userdb.CreateUser("cookieuser", "cookiepass")

	body, _ := json.Marshal(map[string]string{"Login": "cookieuser", "Password": "cookiepass"})
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.Login).ServeHTTP(rr, req)

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
	ctrl := initTestConfig()
	userdb.CreateUser("cookieuser", "cookiepass")

	body, _ := json.Marshal(map[string]string{"Login": "cookieuser", "Password": "cookiepass"})
	req, _ := http.NewRequest("POST", "https://onlogs.example/api/v1/login", bytes.NewBuffer(body))
	req.TLS = &tls.ConnectionState{}
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.Login).ServeHTTP(rr, req)

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("login set no cookie: %s", rr.Body.String())
	}
	if !cookies[0].Secure {
		t.Error("session cookie issued over TLS is not marked Secure")
	}
}

func TestGetLogsStreamRejectsAForeignOrigin(t *testing.T) {
	ctrl := initTestConfig()

	req, _ := http.NewRequest("GET", "/api/v1/getLogsStream?host="+util.GetHost()+"&id=somecontainer", nil)
	req.Host = "onlogs.example"
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})

	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.GetLogsStream).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusForbidden {
		t.Errorf("a cross-origin websocket handshake was not rejected: status %d", code)
	}
	for _, conns := range vars.Connections {
		for _, c := range conns {
			if c == nil {
				t.Fatal("a failed upgrade stored a nil connection that a background goroutine will dereference")
			}
		}
	}
}

func TestLoginRateLimitsRepeatedFailures(t *testing.T) {
	ctrl := initTestConfig()
	userdb.CreateUser("ratelimited", "correct-horse")

	post := func(password string) int {
		body, _ := json.Marshal(map[string]string{"Login": "ratelimited", "Password": password})
		req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
		req.RemoteAddr = "203.0.113.9:34567"
		rr := httptest.NewRecorder()
		http.HandlerFunc(ctrl.Login).ServeHTTP(rr, req)
		return rr.Result().StatusCode
	}

	for i := 0; i < 20; i++ {
		post("wrong-" + strconv.Itoa(i))
	}

	if code := post("correct-horse"); code != http.StatusTooManyRequests {
		t.Fatalf("20 failed logins from one address did not trigger a backoff: status %d", code)
	}
}
