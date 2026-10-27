package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/groups"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestEditUserActuallyChangesThePassword(t *testing.T) {
	os.Setenv("ADMIN_USERNAME", "admin")

	userdb.CreateUser("rotateme", "the-old-password")
	t.Cleanup(func() { userdb.DeleteUser("rotateme") })

	body, _ := json.Marshal(map[string]string{"login": "rotateme", "password": "the-new-password"})
	req, _ := http.NewRequest("POST", "/api/v1/editUser", bytes.NewBuffer(body))
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.EditUser).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("editUser returned %d: %s", code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)
	if response["error"] != nil {
		t.Fatalf("editUser reported an error: %v", response["error"])
	}

	if !userdb.CheckUserPassword("rotateme", "the-new-password") {
		t.Error("editUser reported success but the new password does not work")
	}
	if userdb.CheckUserPassword("rotateme", "the-old-password") {
		t.Error("editUser reported success but the old password still works")
	}
}

func TestEditUserRejectsAnEmptyPassword(t *testing.T) {
	os.Setenv("ADMIN_USERNAME", "admin")

	userdb.CreateUser("emptyrotate", "a-real-password")
	t.Cleanup(func() { userdb.DeleteUser("emptyrotate") })

	body, _ := json.Marshal(map[string]string{"login": "emptyrotate", "password": ""})
	req, _ := http.NewRequest("POST", "/api/v1/editUser", bytes.NewBuffer(body))
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.EditUser).ServeHTTP(rr, req)

	if userdb.CheckUserPassword("emptyrotate", "") {
		t.Fatal("an empty password was accepted")
	}
	if !userdb.CheckUserPassword("emptyrotate", "a-real-password") {
		t.Error("the original password stopped working")
	}
}

func TestCreateUserRejectsCredentialsNobodyCanLogInWith(t *testing.T) {
	for _, c := range []map[string]string{
		{"login": "", "password": "a-password"},
		{"login": " leading-space", "password": "a-password"},
		{"login": "trailing-space ", "password": "a-password"},
		{"login": "empty-password", "password": ""},
	} {
		userdb.DeleteUser(c["login"])
		code, body := call(t, testCtrl.CreateUser, authedRequest(t, "admin", "POST", "/api/v1/createUser", c))
		if code != http.StatusBadRequest || !strings.Contains(string(body), "error") {
			t.Errorf("createUser %q: %d %s", c, code, body)
		}
		if userdb.IsUserExists(c["login"]) {
			userdb.DeleteUser(c["login"])
			t.Errorf("createUser %q stored the account", c)
		}
	}

	userdb.DeleteUser("spaced-password")
	t.Cleanup(func() { userdb.DeleteUser("spaced-password") })
	adminCall(t, testCtrl.CreateUser, "/api/v1/createUser", map[string]string{"login": "spaced-password", "password": " pw with spaces "})
	if !userdb.CheckUserPassword("spaced-password", " pw with spaces ") {
		t.Error("a password with spaces was not stored as typed")
	}
}

func TestEditUserAnswers400ToAnEmptyPassword(t *testing.T) {
	userdb.CreateUser("emptyedit", "a-real-password")
	t.Cleanup(func() { userdb.DeleteUser("emptyedit") })

	code, body := call(t, testCtrl.EditUser, authedRequest(t, "admin", "POST", "/api/v1/editUser", map[string]string{"login": "emptyedit", "password": ""}))
	if code != http.StatusBadRequest || !strings.Contains(string(body), "error") {
		t.Errorf("editUser with an empty password: %d %s", code, body)
	}
	adminCall(t, testCtrl.EditUser, "/api/v1/editUser", map[string]string{"login": "emptyedit", "password": "   "})
	if !userdb.CheckUserPassword("emptyedit", "   ") {
		t.Error("a password of spaces was not stored as typed")
	}
}

func TestGetHostsReadsFavouritesPerHost(t *testing.T) {

	os.RemoveAll("leveldb/hosts")
	os.MkdirAll("leveldb/hosts/FavHostA/containers/shared", 0o700)
	os.MkdirAll("leveldb/hosts/FavHostB/containers/shared", 0o700)

	starred := favouriteKey("someuser", "FavHostB", "shared")
	t.Cleanup(func() {
		vars.FavsDB.Delete(starred, nil)
		os.RemoveAll("leveldb/hosts")
	})

	if err := vars.FavsDB.Put(starred, nil, nil); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/getHosts", nil)
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("someuser")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.GetHosts).ServeHTTP(rr, req)

	var hosts []struct {
		Host     string `json:"host"`
		Services []struct {
			ServiceName string `json:"serviceName"`
			IsFavorite  bool   `json:"isFavorite"`
		} `json:"services"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &hosts); err != nil {
		t.Fatalf("unmarshal: %v -- body %s", err, rr.Body.String())
	}

	favourites := map[string]bool{}
	for _, host := range hosts {
		for _, service := range host.Services {
			favourites[host.Host+"/"+service.ServiceName] = service.IsFavorite
		}
	}

	if !favourites["FavHostB/shared"] {
		t.Error("the starred container on FavHostB is not reported as a favourite")
	}
	if favourites["FavHostA/shared"] {
		t.Error("a container that was never starred on FavHostA is reported as a favourite")
	}
}

func TestFavouritesAreScopedToTheUserWhoSetThem(t *testing.T) {
	userdb.CreateUser("favuser", "favpass")
	userdb.CreateUser("otheruser", "otherpass")
	t.Cleanup(func() {
		userdb.DeleteUser("favuser")
		userdb.DeleteUser("otheruser")
		vars.FavsDB.Delete(favouriteKey("favuser", "h", "svc"), nil)
		vars.FavsDB.Delete(favouriteKey("otheruser", "h", "svc"), nil)
	})
	existingService(t, "h", "svc")

	body, _ := json.Marshal(map[string]string{"host": "h", "service": "svc"})
	req, _ := http.NewRequest("POST", "/api/v1/changeFavorite", bytes.NewBuffer(body))
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("favuser")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(testCtrl.ChangeFavourite).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("changeFavorite returned %d: %s", code, rr.Body.String())
	}

	if starred, _ := vars.FavsDB.Has(favouriteKey("favuser", "h", "svc"), nil); !starred {
		t.Error("the user's own favourite was not recorded")
	}
	if starred, _ := vars.FavsDB.Has(favouriteKey("otheruser", "h", "svc"), nil); starred {
		t.Error("one user's star was applied to another user")
	}
}

func existingService(t *testing.T, host, service string) {
	t.Helper()
	if err := os.MkdirAll("leveldb/hosts/"+host+"/containers/"+service, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll("leveldb/hosts/" + host) })
}

func TestChangeFavoriteRejectsUnsafeNames(t *testing.T) {
	cookie := freshUser(t, "fav-unsafe", "fav-unsafe")
	for _, target := range []map[string]string{
		{"host": "../not-a-host", "service": "x"},
		{"host": "h", "service": ".."},
		{"host": "", "service": "svc"},
	} {
		code, body := call(t, testCtrl.ChangeFavourite, withCookie(t, cookie, "POST", "/", target))
		key := favouriteKey("fav-unsafe", target["host"], target["service"])
		starred, _ := vars.FavsDB.Has(key, nil)
		vars.FavsDB.Delete(key, nil)
		if code != http.StatusBadRequest || starred {
			t.Errorf("changeFavorite %v: %d %s, stored=%v", target, code, body, starred)
		}
	}
}

func TestChangeFavoriteRefusesServicesThatDoNotExist(t *testing.T) {
	cookie := freshUser(t, "fav-bulk", "fav-bulk")
	existingService(t, "h", "svc")
	for _, service := range []string{"svc-000", "svc-001", "svc-002"} {
		code, body := call(t, testCtrl.ChangeFavourite, withCookie(t, cookie, "POST", "/", map[string]string{"host": "h", "service": service}))
		key := favouriteKey("fav-bulk", "h", service)
		starred, _ := vars.FavsDB.Has(key, nil)
		vars.FavsDB.Delete(key, nil)
		if code != http.StatusNotFound || starred {
			t.Errorf("starring h/%s, which does not exist: %d %s, stored=%v", service, code, body, starred)
		}
	}
}

func TestUnstarringWorksAfterTheServiceIsGone(t *testing.T) {
	cookie := freshUser(t, "fav-gone", "fav-gone")
	key := favouriteKey("fav-gone", "h", "removed")
	if err := vars.FavsDB.Put(key, nil, nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.FavsDB.Delete(key, nil) })

	code, body := call(t, testCtrl.ChangeFavourite, withCookie(t, cookie, "POST", "/", map[string]string{"host": "h", "service": "removed"}))
	if code != http.StatusOK || string(body) != "{\"error\":null}\n" {
		t.Fatalf("un-starring: %d %s", code, body)
	}
	if starred, _ := vars.FavsDB.Has(key, nil); starred {
		t.Error("un-starring a service that no longer exists left the favourite in place")
	}
}

func storedUserData(t *testing.T, login string) (list []groups.Group, starred bool, settings map[string]interface{}) {
	t.Helper()
	list, err := groups.List(login)
	if err != nil {
		t.Fatal(err)
	}
	starred, _ = vars.FavsDB.Has(favouriteKey(login, "h", "svc"), nil)
	return list, starred, userdb.GetUserSettings(login)
}

func requireEmptyAccount(t *testing.T, login, cookie string) {
	t.Helper()
	if _, body := call(t, testCtrl.GetGroups, withCookie(t, cookie, "GET", "/", nil)); string(body) != "[]\n" {
		t.Errorf("getGroups of a new account returned %s", body)
	}
	if _, body := call(t, testCtrl.GetUserSettings, withCookie(t, cookie, "GET", "/", nil)); string(body) != "null\n" {
		t.Errorf("getUserSettings of a new account returned %s", body)
	}
	if _, starred, _ := storedUserData(t, login); starred {
		t.Error("a new account starts with a favourite")
	}
}

func TestRecreatedLoginStartsWithoutTheDeletedAccountsData(t *testing.T) {
	const login = "data-recreated"
	old := freshUser(t, login, "first-owner")
	existingService(t, "h", "svc")
	for _, step := range []struct {
		handler http.HandlerFunc
		body    any
	}{
		{testCtrl.CreateGroup, map[string]any{"name": "private", "members": []map[string]string{{"host": "h", "service": "svc"}}}},
		{testCtrl.ChangeFavourite, map[string]string{"host": "h", "service": "svc"}},
		{testCtrl.UpdateUserSettings, map[string]any{"note": "private"}},
	} {
		if code, body := call(t, step.handler, withCookie(t, old, "POST", "/", step.body)); code != http.StatusOK || string(body) != "{\"error\":null}\n" {
			t.Fatalf("seeding the account: %d %s", code, body)
		}
	}
	if list, starred, settings := storedUserData(t, login); len(list) != 1 || !starred || settings == nil {
		t.Fatalf("seeding the account stored groups=%v starred=%v settings=%v", list, starred, settings)
	}

	adminCall(t, testCtrl.DeleteUser, "/api/v1/deleteUser", map[string]string{"login": login})
	if list, starred, settings := storedUserData(t, login); len(list) != 0 || starred || settings != nil {
		t.Errorf("deleting the account left groups=%v starred=%v settings=%v", list, starred, settings)
	}

	adminCall(t, testCtrl.CreateUser, "/api/v1/createUser", map[string]string{"login": login, "password": "second-owner"})
	requireEmptyAccount(t, login, loginCookie(t, login, "second-owner"))
}

func TestNewAccountIgnoresDataLeftByAnEarlierDeletion(t *testing.T) {
	const login = "data-orphaned"
	userdb.DeleteUser(login)
	for _, err := range []error{
		groups.Store(login, "left-behind", []groups.Member{{Host: "h", Service: "svc"}}),
		vars.FavsDB.Put(favouriteKey(login, "h", "svc"), nil, nil),
		userdb.UpdateUserSettings(login, map[string]interface{}{"note": "left-behind"}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}

	adminCall(t, testCtrl.CreateUser, "/api/v1/createUser", map[string]string{"login": login, "password": "new-owner"})
	t.Cleanup(func() { userdb.DeleteUser(login) })
	requireEmptyAccount(t, login, loginCookie(t, login, "new-owner"))
}

func TestRemovingALoginKeepsFavouritesOfALoginThatExtendsItWithNUL(t *testing.T) {
	const login, neighbour = "data-prefix", "data-prefix\x00other"
	cookie := freshUser(t, neighbour, "neighbour")
	existingService(t, "h", "svc")
	if code, body := call(t, testCtrl.ChangeFavourite, withCookie(t, cookie, "POST", "/", map[string]string{"host": "h", "service": "svc"})); code != http.StatusOK || string(body) != "{\"error\":null}\n" {
		t.Fatalf("starring a service: %d %s", code, body)
	}

	userdb.DeleteUser(login)
	adminCall(t, testCtrl.CreateUser, "/api/v1/createUser", map[string]string{"login": login, "password": "owner"})
	adminCall(t, testCtrl.DeleteUser, "/api/v1/deleteUser", map[string]string{"login": login})

	if starred, _ := vars.FavsDB.Has(favouriteKey(neighbour, "h", "svc"), nil); !starred {
		t.Error("removing one login deleted another login's favourite")
	}
}
