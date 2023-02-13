package routes

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
)

func TestFrontend(t *testing.T) {
	os.Mkdir("dist", 0700)
	os.WriteFile("dist/index.html", []byte("text"), 0700)

	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Frontend)
	handler1.ServeHTTP(rr1, req1)
	body1, _ := ioutil.ReadAll(rr1.Result().Body)
	if string(body1) != "text" {
		t.Error("Wrong file content!")
	}

	req2, _ := http.NewRequest("GET", "/fasf", nil)
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(Frontend)
	handler2.ServeHTTP(rr2, req2)
	body2, _ := ioutil.ReadAll(rr2.Result().Body)
	if string(body2) != "text" {
		t.Error("Wrong file content!")
	}
}

func TestCheckCookie(t *testing.T) {
	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(CheckCookie)
	handler1.ServeHTTP(rr1, req1)
	if rr1.Result().StatusCode != 401 {
		t.Error("Should be unauthorized!")
	}

	req2, _ := http.NewRequest("GET", "/", nil)
	req2.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(CheckCookie)
	handler2.ServeHTTP(rr2, req2)
	if rr2.Result().StatusCode != 200 {
		t.Error("Should be unauthorized!")
	}
}

func TestGetHosts(t *testing.T) {
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest1", 0700)
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest2", 0700)
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest3", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest1", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest2", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest3", 0700)
	req1, _ := http.NewRequest("GET", "/frontend", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(GetHosts)
	handler1.ServeHTTP(rr1, req1)
	b, _ := ioutil.ReadAll(rr1.Result().Body)
	if string(b) != "[{\"host\":\"Test1\",\"services\":[{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest1\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest2\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest3\"}]},{\"host\":\"Test2\",\"services\":[{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest1\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest2\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest3\"}]}]" {
		t.Error("Wrong containers or hosts list returned!")
	}
}

func TestSizeByAll(t *testing.T) {
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(GetSizeByAll)
	handler1.ServeHTTP(rr1, req1)
	b, _ := ioutil.ReadAll(rr1.Result().Body)
	if !strings.Contains(string(b), "\"0.0\"") {
		t.Error("Wrong size: ", string(b))
	}
}

func TestSizeByService(t *testing.T) {
	req1, _ := http.NewRequest("GET", "/getSizeByService?service=containerTest1&host=Test1", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(GetSizeByAll)
	handler1.ServeHTTP(rr1, req1)
	b, _ := ioutil.ReadAll(rr1.Result().Body)
	if !strings.Contains(string(b), "\"0.0\"") {
		t.Error("Wrong size: ", string(b))
	}
}

func TestLogin(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testsuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody))
	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Login)
	handler1.ServeHTTP(rr1, req1)
	b, _ := ioutil.ReadAll(rr1.Result().Body)
	if !strings.Contains(string(b), "Wrong") {
		t.Error("Password must be wrong!")
	}

	postBody2, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req2, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody2))
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(Login)
	handler2.ServeHTTP(rr2, req2)
	b2, _ := ioutil.ReadAll(rr2.Result().Body)
	if !strings.Contains(string(b2), "null") {
		t.Error("Password must be wrong!")
	}
}

func TestLogout(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody))
	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Login)
	handler1.ServeHTTP(rr1, req1)

	rr2 := httptest.NewRecorder()
	req1.AddCookie(rr1.Result().Cookies()[0])
	handler2 := http.HandlerFunc(Logout)
	handler2.ServeHTTP(rr2, req1)

	if rr2.Result().Cookies()[0].Value != "toDelete" {
		t.Error("Wrong cookie value!")
	}
}
