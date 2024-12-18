package routes

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/joho/godotenv"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestFrontend(t *testing.T) {
	os.Mkdir("dist", 0700)
	os.WriteFile("dist/index.html", []byte("text"), 0700)

	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Frontend)
	handler1.ServeHTTP(rr1, req1)
	body1, _ := io.ReadAll(rr1.Result().Body)
	if string(body1) != "text" {
		t.Error("Wrong file content!")
	}

	req2, _ := http.NewRequest("GET", "/fasf", nil)
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(Frontend)
	handler2.ServeHTTP(rr2, req2)
	body2, _ := io.ReadAll(rr2.Result().Body)
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
	err := godotenv.Load("../../.env")
	if err != nil {
		os.Setenv("DOCKER_SOCKET_PATH", "/var/run/docker.sock")
	}

	os.RemoveAll("leveldb/hosts")
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
	b, _ := io.ReadAll(rr1.Result().Body)
	if string(b) != "[{\"host\":\"Test1\",\"services\":[{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest1\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest2\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest3\"}]},{\"host\":\"Test2\",\"services\":[{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest1\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest2\"},{\"isDisabled\":true,\"isFavorite\":false,\"serviceName\":\"containerTest3\"}]},{\"host\":\""+util.GetHost()+"\",\"services\":[]}]" {
		t.Error("Wrong containers or hosts list returned!\n" + string(b))
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
	b, _ := io.ReadAll(rr1.Result().Body)
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
	b, _ := io.ReadAll(rr1.Result().Body)
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
	b, _ := io.ReadAll(rr1.Result().Body)
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
	b2, _ := io.ReadAll(rr2.Result().Body)
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

func TestGetStats(t *testing.T) {
	postBody1, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody1))
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Login)
	handler1.ServeHTTP(rr1, req1)
	rr2 := httptest.NewRecorder()

	vars.Container_Stat_Counter["test/test"] = map[string]uint64{"error": 1, "debug": 2, "info": 3, "warn": 4, "meta": 0, "other": 5}
	os.RemoveAll("leveldb/hosts/test/containers/test/statistics")
	statDB, _ := leveldb.OpenFile("leveldb/hosts/test/containers/test/statistics", nil)
	to_put, _ := json.Marshal(vars.Container_Stat_Counter["test/test"])
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	statDB.Put([]byte(datetime), to_put, nil)
	statDB.Close()

	postBody2, _ := json.Marshal(map[string]interface{}{
		"host":    "test",
		"service": "test",
		"period":  2,
	})
	req2, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody2))
	req2.AddCookie(rr1.Result().Cookies()[0])
	handler2 := http.HandlerFunc(GetStats)
	handler2.ServeHTTP(rr2, req2)

	b, _ := io.ReadAll(rr2.Result().Body)
	res := map[string]int{}
	json.Unmarshal(b, &res)
	if res["debug"] != 4 || res["error"] != 2 ||
		res["info"] != 6 || res["other"] != 10 ||
		res["warn"] != 8 {
		t.Error("Wrong value!\n", res)
	}
}

func TestGetChartData(t *testing.T) {
	postBody1, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody1))
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(Login)
	handler1.ServeHTTP(rr1, req1)

	cur_db, _ := leveldb.OpenFile("leveldb/hosts/test/statistics", nil)
	vars.Stat_Hosts_DBs["test"] = cur_db
	vars.Container_Stat_Counter["test/test"] = map[string]uint64{"error": 2, "debug": 1, "info": 3, "warn": 5, "meta": 0, "other": 4}
	vars.Stat_Containers_DBs["test/test"] = cur_db
	to_put, _ := json.Marshal(vars.Container_Stat_Counter["test/test"])
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	cur_db.Put([]byte(datetime), to_put, nil)

	rr2 := httptest.NewRecorder()
	postBody2, _ := json.Marshal(map[string]interface{}{
		"host":        "test",
		"service":     "test",
		"unit":        "hour",
		"unitsAmount": 2,
	})
	req2, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody2))
	req2.AddCookie(rr1.Result().Cookies()[0])
	handler2 := http.HandlerFunc(GetChartData)
	handler2.ServeHTTP(rr2, req2)

	res := map[string]map[string]int{}
	b, _ := io.ReadAll(rr2.Body)
	json.Unmarshal(b, &res)
	datetime = datetime[:len(datetime)-6] + "00Z"
	if res[datetime]["debug"] != 1 || res[datetime]["error"] != 2 ||
		res[datetime]["info"] != 3 || res[datetime]["other"] != 4 ||
		res[datetime]["warn"] != 5 || res["now"]["debug"] != 1 ||
		res["now"]["error"] != 2 || res["now"]["info"] != 3 ||
		res["now"]["other"] != 4 || res["now"]["warn"] != 5 {
		t.Error("Wrong value!\n", res[datetime])
	}
}
