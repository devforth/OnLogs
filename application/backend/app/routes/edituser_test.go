package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestEditUserActuallyChangesThePassword(t *testing.T) {
	ctrl := initTestConfig()
	os.Setenv("ADMIN_USERNAME", "admin")

	userdb.CreateUser("rotateme", "the-old-password")
	t.Cleanup(func() { userdb.DeleteUser("rotateme", "") })

	body, _ := json.Marshal(map[string]string{"login": "rotateme", "password": "the-new-password"})
	req, _ := http.NewRequest("POST", "/api/v1/editUser", bytes.NewBuffer(body))
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.EditUser).ServeHTTP(rr, req)

	if code := rr.Result().StatusCode; code != http.StatusOK {
		t.Fatalf("editUser returned %d: %s", code, rr.Body.String())
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)
	if response["error"] != nil {
		t.Fatalf("editUser reported an error: %v", response["error"])
	}

	// The UI shows "Password was changed" on this response, so it had better be true.
	if !userdb.CheckUserPassword("rotateme", "the-new-password") {
		t.Error("editUser reported success but the new password does not work")
	}
	if userdb.CheckUserPassword("rotateme", "the-old-password") {
		t.Error("editUser reported success but the old password still works")
	}
}

func TestEditUserRejectsAnEmptyPassword(t *testing.T) {
	ctrl := initTestConfig()
	os.Setenv("ADMIN_USERNAME", "admin")

	userdb.CreateUser("emptyrotate", "a-real-password")
	t.Cleanup(func() { userdb.DeleteUser("emptyrotate", "") })

	body, _ := json.Marshal(map[string]string{"login": "emptyrotate", "password": ""})
	req, _ := http.NewRequest("POST", "/api/v1/editUser", bytes.NewBuffer(body))
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("admin")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.EditUser).ServeHTTP(rr, req)

	if userdb.CheckUserPassword("emptyrotate", "") {
		t.Fatal("an empty password was accepted")
	}
	if !userdb.CheckUserPassword("emptyrotate", "a-real-password") {
		t.Error("the original password stopped working")
	}
}

// Favourites are stored as "<host>/<service>", but GetHosts looked every one up
// under the LOCAL hostname, so stars vanished on remote hosts and appeared on
// containers that were never starred.
func TestGetHostsReadsFavouritesPerHost(t *testing.T) {
	ctrl := initTestConfig()

	os.RemoveAll("leveldb/hosts")
	os.MkdirAll("leveldb/hosts/FavHostA/containers/shared", 0o700)
	os.MkdirAll("leveldb/hosts/FavHostB/containers/shared", 0o700)
	t.Cleanup(func() {
		vars.FavsDB.Delete([]byte("FavHostB/shared"), nil)
		os.RemoveAll("leveldb/hosts")
	})

	if err := vars.FavsDB.Put([]byte("FavHostB/shared"), nil, nil); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest("GET", "/api/v1/getHosts", nil)
	req.AddCookie(&http.Cookie{Name: "onlogs-cookie", Value: util.CreateJWT("someuser")})
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.GetHosts).ServeHTTP(rr, req)

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
