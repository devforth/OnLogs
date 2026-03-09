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

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
	"github.com/syndtr/goleveldb/leveldb"
)

func initTestConfig() *RouteController {
	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	defer cli.Close()

	dockerService := &docker.DockerService{
		Client: cli,
	}

	daemonService := &daemon.DaemonService{
		DockerClient: dockerService,
	}

	// Initialize the "Controller" with its dependencies
	routerCtrl := &RouteController{
		DockerService: dockerService,
		DaemonService: daemonService,
	}
	return routerCtrl
}

func TestFrontend(t *testing.T) {
	ctrl := initTestConfig()
	os.Mkdir("dist", 0700)
	os.WriteFile("dist/index.html", []byte("text"), 0700)

	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.Frontend)
	handler1.ServeHTTP(rr1, req1)
	body1, _ := io.ReadAll(rr1.Result().Body)
	if string(body1) != "text" {
		t.Error("Wrong file content!")
	}

	req2, _ := http.NewRequest("GET", "/fasf", nil)
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(ctrl.Frontend)
	handler2.ServeHTTP(rr2, req2)
	body2, _ := io.ReadAll(rr2.Result().Body)
	if string(body2) != "text" {
		t.Error("Wrong file content!")
	}
}

func TestCheckCookie(t *testing.T) {
	ctrl := initTestConfig()

	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.CheckCookie)
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
	handler2 := http.HandlerFunc(ctrl.CheckCookie)
	handler2.ServeHTTP(rr2, req2)
	if rr2.Result().StatusCode != 200 {
		t.Error("Should be unauthorized!")
	}
}

func TestGetHosts(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}
	ctrl := initTestConfig()

	os.RemoveAll("leveldb/hosts")
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest1", 0700)
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest2", 0700)
	os.MkdirAll("leveldb/hosts/Test1/containers/containerTest3", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest1", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest2", 0700)
	os.MkdirAll("leveldb/hosts/Test2/containers/containerTest3", 0700)
	os.MkdirAll("leveldb/hosts/"+util.GetHost()+"/containers", 0700)
	req1, _ := http.NewRequest("GET", "/frontend", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})

	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.GetHosts)
	handler1.ServeHTTP(rr1, req1)
	b, _ := io.ReadAll(rr1.Result().Body)

	type service struct {
		IsDisabled  bool   `json:"isDisabled"`
		IsFavorite  bool   `json:"isFavorite"`
		ServiceName string `json:"serviceName"`
	}
	type hostEntry struct {
		Host     string    `json:"host"`
		Services []service `json:"services"`
	}

	var hosts []hostEntry
	if err := json.Unmarshal(b, &hosts); err != nil {
		t.Fatalf("failed to unmarshal response: %v -- body: %s", err, string(b))
	}

	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d -- body: %s", len(hosts), string(b))
	}

	// Build lookup map for hosts
	hostMap := make(map[string]hostEntry)
	for _, h := range hosts {
		hostMap[h.Host] = h
	}

	expectedHosts := []string{util.GetHost(), "Test1", "Test2"}
	expectedServices := []string{"containerTest1", "containerTest2", "containerTest3"}

	for _, eh := range expectedHosts {
		he, ok := hostMap[eh]
		if !ok {
			t.Errorf("missing host %s", eh)
			continue
		}
		if eh == util.GetHost() {
			if len(he.Services) != 0 {
				t.Errorf("expected no services for host %s, got %v", eh, he.Services)
			}
			continue
		}
		// For Test1/Test2 ensure all expected services are present (order-independent)
		svcSet := make(map[string]bool)
		for _, s := range he.Services {
			svcSet[s.ServiceName] = true
		}
		for _, es := range expectedServices {
			if !svcSet[es] {
				t.Errorf("host %s missing service %s (services: %v)", eh, es, he.Services)
			}
		}
	}
}

func TestSizeByAll(t *testing.T) {
	ctrl := initTestConfig()
	req1, _ := http.NewRequest("GET", "/", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.GetSizeByAll)
	handler1.ServeHTTP(rr1, req1)
	b, _ := io.ReadAll(rr1.Result().Body)
	if !strings.Contains(string(b), "\"0.0\"") {
		t.Error("Wrong size: ", string(b))
	}
}

func TestSizeByService(t *testing.T) {
	ctrl := initTestConfig()
	req1, _ := http.NewRequest("GET", "/getSizeByService?service=containerTest1&host=Test1", nil)
	req1.AddCookie(&http.Cookie{
		Name:  "onlogs-cookie",
		Value: util.CreateJWT("testuser"),
	})
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.GetSizeByAll)
	handler1.ServeHTTP(rr1, req1)
	b, _ := io.ReadAll(rr1.Result().Body)
	if !strings.Contains(string(b), "\"0.0\"") {
		t.Error("Wrong size: ", string(b))
	}
}

func TestLogin(t *testing.T) {
	ctrl := initTestConfig()
	postBody, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testsuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody))
	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.Login)
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
	handler2 := http.HandlerFunc(ctrl.Login)
	handler2.ServeHTTP(rr2, req2)
	b2, _ := io.ReadAll(rr2.Result().Body)
	if !strings.Contains(string(b2), "null") {
		t.Error("Password must be wrong!")
	}
}

func TestLogout(t *testing.T) {
	ctrl := initTestConfig()
	postBody, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody))
	userdb.CreateUser("testuser", "testuser")

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.Login)
	handler1.ServeHTTP(rr1, req1)

	rr2 := httptest.NewRecorder()
	req1.AddCookie(rr1.Result().Cookies()[0])
	handler2 := http.HandlerFunc(ctrl.Logout)
	handler2.ServeHTTP(rr2, req1)

	if rr2.Result().Cookies()[0].Value != "toDelete" {
		t.Error("Wrong cookie value!")
	}
}

func TestGetStats(t *testing.T) {
	ctrl := initTestConfig()
	postBody1, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody1))
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.Login)
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
	handler2 := http.HandlerFunc(ctrl.GetStats)
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
	ctrl := initTestConfig()
	postBody1, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody1))
	userdb.CreateUser("testuser", "testuser")
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(ctrl.Login)
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
	handler2 := http.HandlerFunc(ctrl.GetChartData)
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
