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

var testCtrl *RouteController

func TestMain(m *testing.M) {
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "routes-package-test-signing-key")
	}
	// A token is only accepted while its account exists, so the accounts these
	// tests sign tokens for have to be present.
	os.Setenv("ADMIN_USERNAME", "admin")
	userdb.CreateUser("admin", "admin-password-for-tests")
	userdb.CreateUser("testuser", "testuser")
	userdb.CreateUser("viewer", "viewer-password")
	userdb.CreateUser("someuser", "someuser-password")

	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	dockerService := &docker.DockerService{Client: cli}
	testCtrl = &RouteController{
		DockerService: dockerService,
		DaemonService: daemon.NewDaemonService(dockerService),
	}

	code := m.Run()
	cli.Close()
	os.Exit(code)
}

func TestFrontend(t *testing.T) {
	os.Mkdir("dist", 0700)
	os.WriteFile("dist/index.html", []byte("text"), 0700)

	req1, _ := http.NewRequest("GET", "/frontend", nil)
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(testCtrl.Frontend)
	handler1.ServeHTTP(rr1, req1)
	body1, _ := io.ReadAll(rr1.Result().Body)
	if string(body1) != "text" {
		t.Error("Wrong file content!")
	}

	req2, _ := http.NewRequest("GET", "/fasf", nil)
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(testCtrl.Frontend)
	handler2.ServeHTTP(rr2, req2)
	body2, _ := io.ReadAll(rr2.Result().Body)
	if string(body2) != "text" {
		t.Error("Wrong file content!")
	}
}

func TestCheckCookie(t *testing.T) {
	if status, _ := call(t, testCtrl.CheckCookie, authedRequest(t, "", "GET", "/", nil)); status != 401 {
		t.Errorf("a request with no cookie returned %d, want 401", status)
	}
	if status, _ := call(t, testCtrl.CheckCookie, authedRequest(t, "testuser", "GET", "/", nil)); status != 200 {
		t.Errorf("a signed request returned %d, want 200", status)
	}
}

func TestGetHosts(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}

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

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(testCtrl.GetHosts)
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

func TestSizeEndpointsReportAnEmptyStore(t *testing.T) {
	cases := []struct {
		name    string
		handler http.HandlerFunc
		target  string
	}{
		{"sizeByAll", testCtrl.GetSizeByAll, "/api/v1/getSizeByAll"},
		{"sizeByService", testCtrl.GetSizeByService, "/api/v1/getSizeByService?service=containerTest1&host=Test1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, body := call(t, c.handler, authedRequest(t, "testuser", "GET", c.target, nil))
			if !strings.Contains(string(body), "\"0.0\"") {
				t.Errorf("%s reported %s, want 0.0", c.name, body)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	_, wrong := call(t, testCtrl.Login,
		authedRequest(t, "", "POST", "/", map[string]string{"Login": "testuser", "Password": "testsuser"}))
	if !strings.Contains(string(wrong), "Wrong") {
		t.Errorf("a bad password was accepted: %s", wrong)
	}

	_, right := call(t, testCtrl.Login,
		authedRequest(t, "", "POST", "/", map[string]string{"Login": "testuser", "Password": "testuser"}))
	if !strings.Contains(string(right), "null") {
		t.Errorf("the correct password was rejected: %s", right)
	}
}

func TestLogout(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{
		"Login":    "testuser",
		"Password": "testuser",
	})
	req1, _ := http.NewRequest("POST", "/", bytes.NewBuffer(postBody))

	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(testCtrl.Login)
	handler1.ServeHTTP(rr1, req1)

	rr2 := httptest.NewRecorder()
	req1.AddCookie(rr1.Result().Cookies()[0])
	handler2 := http.HandlerFunc(testCtrl.Logout)
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
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(testCtrl.Login)
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
	handler2 := http.HandlerFunc(testCtrl.GetStats)
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
	rr1 := httptest.NewRecorder()
	handler1 := http.HandlerFunc(testCtrl.Login)
	handler1.ServeHTTP(rr1, req1)

	cur_db, _ := leveldb.OpenFile("leveldb/hosts/test/statistics", nil)
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
	handler2 := http.HandlerFunc(testCtrl.GetChartData)
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
