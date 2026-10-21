package routes

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
)

func loginCookie(t *testing.T, login, password string) string {
	t.Helper()
	rr := httptest.NewRecorder()
	testCtrl.Login(rr, authedRequest(t, "", "POST", "/api/v1/login", map[string]string{"login": login, "password": password}))
	for _, c := range rr.Result().Cookies() {
		if c.Name == "onlogs-cookie" {
			return c.Value
		}
	}
	t.Fatalf("login as %s failed: %s", login, rr.Body.String())
	return ""
}

func withCookie(t *testing.T, cookie, method, target string, body any) *http.Request {
	t.Helper()
	req := authedRequest(t, "", method, target, body)
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: cookie})
	return req
}

func cookieStatus(t *testing.T, cookie string) int {
	t.Helper()
	code, _ := call(t, testCtrl.CheckCookie, withCookie(t, cookie, "GET", "/api/v1/checkCookie", nil))
	return code
}

func freshUser(t *testing.T, login, password string) string {
	t.Helper()
	userdb.DeleteUser(login)
	if err := userdb.CreateUser(login, password); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { userdb.DeleteUser(login) })
	cookie := loginCookie(t, login, password)
	if code := cookieStatus(t, cookie); code != http.StatusOK {
		t.Fatalf("a fresh session was rejected: %d", code)
	}
	return cookie
}

func adminCall(t *testing.T, handler http.HandlerFunc, target string, body any) {
	t.Helper()
	code, resp := call(t, handler, authedRequest(t, "admin", "POST", target, body))
	if code != http.StatusOK || string(resp) != "{\"error\":null}\n" {
		t.Fatalf("%s: %d %s", target, code, resp)
	}
}

func TestPasswordChangeEndsExistingSessions(t *testing.T) {
	old := freshUser(t, "session-rotated", "old-password")
	bystander := freshUser(t, "session-bystander", "bystander-password")

	adminCall(t, testCtrl.EditUser, "/api/v1/editUser", map[string]string{"login": "session-rotated", "password": "new-password"})

	if code := cookieStatus(t, old); code != http.StatusUnauthorized {
		t.Errorf("a session issued before the password change got %d", code)
	}
	if code := cookieStatus(t, loginCookie(t, "session-rotated", "new-password")); code != http.StatusOK {
		t.Errorf("a session issued after the password change got %d", code)
	}
	if code := cookieStatus(t, bystander); code != http.StatusOK {
		t.Errorf("another user's session got %d", code)
	}
}

func TestLogoutEndsTheSessionServerSide(t *testing.T) {
	cookie := freshUser(t, "session-logout", "logout-password")

	if code, body := call(t, testCtrl.Logout, withCookie(t, cookie, "GET", "/api/v1/logout", nil)); code != http.StatusOK {
		t.Fatalf("logout: %d %s", code, body)
	}

	if code := cookieStatus(t, cookie); code != http.StatusUnauthorized {
		t.Errorf("the logged-out cookie got %d", code)
	}
	if code := cookieStatus(t, loginCookie(t, "session-logout", "logout-password")); code != http.StatusOK {
		t.Errorf("a new login after logout got %d", code)
	}
}

func TestLoggedOutAdminCookieLosesAdminRoutes(t *testing.T) {
	adm := loginCookie(t, "admin", "admin-password-for-tests")
	if code, body := call(t, testCtrl.Logout, withCookie(t, adm, "GET", "/api/v1/logout", nil)); code != http.StatusOK {
		t.Fatalf("logout: %d %s", code, body)
	}

	userdb.DeleteUser("session-planted")
	t.Cleanup(func() { userdb.DeleteUser("session-planted") })
	code, _ := call(t, testCtrl.CreateUser, withCookie(t, adm, "POST", "/api/v1/createUser", map[string]string{"login": "session-planted", "password": "planted"}))
	if code == http.StatusOK || userdb.IsUserExists("session-planted") {
		t.Errorf("the logged-out admin cookie created a user (status %d)", code)
	}
	if code, body := call(t, testCtrl.GetSecret, withCookie(t, adm, "GET", "/api/v1/getSecret", nil)); code == http.StatusOK {
		t.Errorf("the logged-out admin cookie minted an agent token: %s", body)
	}
}

func TestRecreatedAccountRejectsSessionsOfTheDeletedOne(t *testing.T) {
	old := freshUser(t, "session-recreated", "first-owner")

	adminCall(t, testCtrl.DeleteUser, "/api/v1/deleteUser", map[string]string{"login": "session-recreated"})
	if code := cookieStatus(t, old); code != http.StatusUnauthorized {
		t.Fatalf("a session of a deleted account got %d", code)
	}

	adminCall(t, testCtrl.CreateUser, "/api/v1/createUser", map[string]string{"login": "session-recreated", "password": "second-owner"})
	if code := cookieStatus(t, old); code != http.StatusUnauthorized {
		t.Errorf("a session of the deleted account got %d after the login was re-created", code)
	}
	if code := cookieStatus(t, loginCookie(t, "session-recreated", "second-owner")); code != http.StatusOK {
		t.Errorf("a session of the re-created account got %d", code)
	}
}

func openStream(t *testing.T, server *httptest.Server, container, cookie string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/getLogsStream?id=" + container + "&host=" + util.GetHost()
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Cookie": {"onlogs-cookie=" + cookie}})
	if err != nil {
		t.Fatalf("opening the stream: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestRevokingSessionsClosesOpenLogStreams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(testCtrl.GetLogsStream))
	defer server.Close()

	for name, revoke := range map[string]func(t *testing.T, cookie string){
		"delete": func(t *testing.T, _ string) {
			adminCall(t, testCtrl.DeleteUser, "/api/v1/deleteUser", map[string]string{"login": "stream-revoked"})
		},
		"password-change": func(t *testing.T, _ string) {
			adminCall(t, testCtrl.EditUser, "/api/v1/editUser", map[string]string{"login": "stream-revoked", "password": "changed"})
		},
		"logout": func(t *testing.T, cookie string) {
			call(t, testCtrl.Logout, withCookie(t, cookie, "GET", "/api/v1/logout", nil))
		},
	} {
		t.Run(name, func(t *testing.T) {
			container := "stream-" + name
			cookie := freshUser(t, "stream-revoked", "stream-password")
			revoked := openStream(t, server, container, cookie)
			kept := openStream(t, server, container, freshUser(t, "stream-bystander", "bystander-password"))
			for deadline := time.Now().Add(2 * time.Second); vars.ConnectionCount(container) < 2; time.Sleep(time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatalf("streams not registered: %d", vars.ConnectionCount(container))
				}
			}

			revoke(t, cookie)
			vars.Broadcast(container, websocket.TextMessage, []byte("after revocation"))

			kept.SetReadDeadline(time.Now().Add(2 * time.Second))
			if _, msg, err := kept.ReadMessage(); err != nil || string(msg) != "after revocation" {
				t.Fatalf("a still-valid user's stream got %q, %v", msg, err)
			}
			revoked.SetReadDeadline(time.Now().Add(2 * time.Second))
			if _, msg, err := revoked.ReadMessage(); err == nil {
				t.Errorf("the revoked user's stream received %q", msg)
			}
		})
	}
}

type hijackHook struct {
	http.ResponseWriter
	before func()
}

func (h hijackHook) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.before()
	return h.ResponseWriter.(http.Hijacker).Hijack()
}

func TestStreamRevokedDuringTheUpgradeIsClosed(t *testing.T) {
	for name, revoke := range map[string]func(login string) error{
		"delete": userdb.DeleteUser,
		"revoke": userdb.RevokeSessions,
	} {
		t.Run(name, func(t *testing.T) {
			cookie := freshUser(t, "stream-upgrading", "upgrading-password")
			done := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(done)
				testCtrl.GetLogsStream(hijackHook{w, func() { revoke("stream-upgrading") }}, r)
			}))
			defer server.Close()

			container := "stream-upgrading-" + name
			conn := openStream(t, server, container, cookie)
			<-done
			if n := vars.ConnectionCount(container); n != 0 {
				t.Errorf("the revoked stream is still registered (%d)", n)
			}

			vars.Broadcast(container, websocket.TextMessage, []byte("after revocation"))
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if _, msg, err := conn.ReadMessage(); err == nil {
				t.Errorf("the revoked stream received %q", msg)
			}
		})
	}
}

func TestStreamWithoutASessionWorksWhenAuthIsDisabled(t *testing.T) {
	t.Setenv("DISABLE_AUTH", "true")
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(done)
		testCtrl.GetLogsStream(w, r)
	}))
	defer server.Close()

	conn := openStream(t, server, "stream-no-auth", "")
	<-done
	vars.Broadcast("stream-no-auth", websocket.TextMessage, []byte("line"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "line" {
		t.Fatalf("the stream got %q, %v", msg, err)
	}
}
