package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

func enableCors(w *http.ResponseWriter) {
	var origin string
	if os.Getenv("ENV_NAME") == "local" {
		origin = "http://localhost:5173"
	} else {
		origin = ""
	}
	(*w).Header().Set("Access-Control-Allow-Origin", origin)
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func verifyAdminUser(w *http.ResponseWriter, req *http.Request) bool {
	username, err := util.GetUserFromJWT(*req)
	if username != "admin" {
		(*w).WriteHeader(http.StatusForbidden)
		json.NewEncoder(*w).Encode(map[string]string{"error": "Only admin can perform this request"})
		return false
	}

	if err != nil {
		(*w).WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(*w).Encode(map[string]string{"error": err.Error()})
		return false
	}
	return true
}

func verifyUser(w *http.ResponseWriter, req *http.Request) bool {
	_, err := util.GetUserFromJWT(*req)
	if err != nil {
		(*w).WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(*w).Encode(map[string]string{"error": err.Error()})
		return false
	}
	return true
}

func verifyRequest(w *http.ResponseWriter, req *http.Request) bool {
	enableCors(w)
	if req.Method == "OPTIONS" {
		(*w).WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func Frontend(w http.ResponseWriter, req *http.Request) {
	requestedPath := strings.ReplaceAll(req.URL.String(), os.Getenv("ONLOGS_PATH_PREFIX"), "")

	dirPath, fileName := filepath.Split(requestedPath)
	if fileName == "" {
		fileName = "index.html"
	}

	fileName = strings.Split(fileName, "?")[0]
	dir := http.Dir("dist" + dirPath)
	file, err := dir.Open(fileName)
	if err != nil {
		dir = http.Dir("dist")
		file, err = dir.Open("index.html")
	}
	if err != nil {
		return
	}
	defer file.Close()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(fileName)))

	stat, _ := file.Stat()
	content := make([]byte, stat.Size())
	io.ReadFull(file, content)
	http.ServeContent(w, req, requestedPath, stat.ModTime(), bytes.NewReader(content))
}

func CheckCookie(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func AddLogLine(w http.ResponseWriter, req *http.Request) {
	var logItem struct {
		Host      string
		Container string
		LogLine   []string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)
	if vars.Counters_For_Hosts_Last_30_Min[logItem.Host] == nil {
		go statistics.RunStatisticForContainer(logItem.Host, logItem.Container)
	}
	location := logItem.Host + "/" + logItem.Container
	if vars.Statuses_DBs[location] == nil {
		vars.Statuses_DBs[location] = util.GetDB(logItem.Host, logItem.Container, "statuses")
	}
	current_db, _ := leveldb.OpenFile("leveldb/hosts/"+logItem.Host+"/containers/"+logItem.Container+"/logs", nil)
	containerdb.PutLogMessage(current_db, logItem.Host, logItem.Container, logItem.LogLine)
	defer current_db.Close()

	to_send, _ := json.Marshal([]string{logItem.LogLine[0], logItem.LogLine[1]})
	for _, c := range vars.Connections[location] {
		c.WriteMessage(1, to_send)
	}
}

func AddHost(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var addReq struct {
		Hostname string
		Token    string
		Services []string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&addReq)

	if !db.IsTokenExists(addReq.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	vars.AgentsActiveContainers[addReq.Hostname] = addReq.Services
	for _, container := range addReq.Services {
		os.MkdirAll("leveldb/hosts/"+addReq.Hostname+"/containers/"+container, 0700)
	}
}

func ChangeFavourite(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var container struct {
		Host    string `json:"host"`
		Service string `json:"service"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&container)

	key := []byte(container.Host + "/" + container.Service)
	isAlreadyFavourite, _ := vars.FavsDB.Has(key, nil)
	if isAlreadyFavourite {
		vars.FavsDB.Delete(key, nil)
	} else {
		vars.FavsDB.Put(key, nil, nil)
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func GetSecret(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": db.CreateOnLogsToken()})
}

func GetChartData(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var data struct {
		Host        string `json:"host"`
		Service     string `json:"service"`
		Unit        string `json:"unit"`
		UnitsAmount int    `json:"unitsAmount"`
	}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)
	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid data!"})
		return
	}

	if !util.Contains(data.Unit, []string{"hour", "day", "month"}) {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid data!"})
	}

	w.Header().Add("Content-Type", "application/json")
	e, _ := json.Marshal(
		statistics.GetChartData(
			data.Host, data.Service, data.Unit, data.UnitsAmount,
		))
	w.Write(e)
}

func GetHosts(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	type HostsList struct {
		Host     string                   `json:"host"`
		Services []map[string]interface{} `json:"services"`
	}

	var to_return []HostsList

	activeContainers := daemon.GetContainersList()

	hosts, _ := os.ReadDir("leveldb/hosts/")
	for _, host := range hosts {
		containers, _ := os.ReadDir("leveldb/hosts/" + host.Name() + "/containers")
		allContainers := []map[string]interface{}{}
		for _, container := range containers {
			isFavorite, _ := vars.FavsDB.Has([]byte(util.GetHost()+"/"+container.Name()), nil)
			if util.Contains(container.Name(), activeContainers) || util.Contains(container.Name(), vars.AgentsActiveContainers[host.Name()]) {
				allContainers = append(allContainers, map[string]interface{}{"serviceName": container.Name(), "isDisabled": false, "isFavorite": isFavorite})
			} else {
				allContainers = append(allContainers, map[string]interface{}{"serviceName": container.Name(), "isDisabled": true, "isFavorite": isFavorite})
			}
		}
		to_return = append(to_return, HostsList{Host: host.Name(), Services: allContainers})
	}

	w.Header().Add("Content-Type", "application/json")
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func GetSizeByAll(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var totalSize float64
	hosts, _ := os.ReadDir("leveldb/hosts/")
	for _, host := range hosts {
		containers, _ := os.ReadDir("leveldb/hosts/" + host.Name() + "/containers")
		for _, container := range containers {
			totalSize += util.GetDirSize(host.Name(), container.Name())
		}
	}

	if totalSize < 0.1 && totalSize != 0.0 {
		totalSize = 0.1
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", totalSize)}) // MiB
}

// TODO need to return 0.0 when there is no logs for container in db
func GetSizeByService(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	if params.Get("service") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	w.Header().Add("Content-Type", "application/json")

	size := util.GetDirSize(params.Get("host"), params.Get("service"))
	if size < 0.1 && size != 0.0 {
		size = 0.1
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", size)}) // MiB
}

func GetStats(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var data struct {
		Host    string `json:"host"`
		Service string `json:"service"`
		Value   int    `json:"period"` // 1 = 30min, 2 = 1hr, 48 = 1d
	}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)

	if err != nil {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid data!"})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statistics.GetStatisticsByService(data.Host, data.Service, data.Value))
}

func GetPrevLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	caseSensetive, err := strconv.ParseBool(params.Get("caseSens"))
	if err != nil {
		caseSensetive = false
	}

	if params.Get("startWith") == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Need to specify \"startWith\"!"})
		return
	}
	w.Header().Add("Content-Type", "application/json")

	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	json.NewEncoder(w).Encode(containerdb.GetLogs(true, false, params.Get("host"), params.Get("id"), params.Get("search"), limit, params.Get("startWith"), caseSensetive))
}

func GetLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	caseSensetive, err := strconv.ParseBool(params.Get("caseSens"))
	if err != nil {
		caseSensetive = false
	}
	w.Header().Add("Content-Type", "application/json")
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}

	if params.Get("status") != "" {
		json.NewEncoder(w).Encode(containerdb.GetLogsByStatus(
			params.Get("host"), params.Get("id"), params.Get("message"), params.Get("status"),
			limit, params.Get("startWith"), false, true, caseSensetive,
		))
	} else {
		json.NewEncoder(w).Encode(containerdb.GetLogs(
			false, false, params.Get("host"), params.Get("id"), params.Get("search"),
			limit, params.Get("startWith"), caseSensetive,
		))
	}
}

func GetLogWithPrev(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	w.Header().Add("Content-Type", "application/json")
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	json.NewEncoder(w).Encode(containerdb.GetLogs(false, true, params.Get("host"), params.Get("id"), "", limit, params.Get("startWith"), false))
}

// TODO return {"error": "Invalid host!"} when host is not exists
func GetLogsStream(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	container := req.URL.Query().Get("id")
	if container == "" {
		return
	}

	host := req.URL.Query().Get("host")

	if host == "" {
		panic("Host is not mentioned!")
	}
	if host != util.GetHost() {
		container = host + "/" + container
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true } // verify req here?

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		fmt.Println(err)
	}
	vars.Connections[container] = append(vars.Connections[container], *ws)
}

func Login(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var loginData vars.UserData
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)

	isCorrect := userdb.CheckUserPassword(loginData.Login, loginData.Password)
	if !isCorrect {
		json.NewEncoder(w).Encode(map[string]string{"error": "Wrong login or password!"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "onlogs-cookie",
		Value:    util.CreateJWT(loginData.Login),
		Expires:  time.Now().AddDate(0, 0, 2),
		MaxAge:   int(time.Now().AddDate(0, 0, 2).Unix()),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func Logout(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "onlogs-cookie",
		Value:    "toDelete",
		Expires:  time.Now().AddDate(-5, -5, -5),
		MaxAge:   0,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func CreateUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var loginData vars.UserData
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)

	err := userdb.CreateUser(loginData.Login, loginData.Password)
	if err == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func GetUsers(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	users := userdb.GetUsers()
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"users": users})
}

func UpdateUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var settings map[string]interface{}
	body, _ := ioutil.ReadAll(req.Body)
	json.Unmarshal(body, &settings)
	username, _ := util.GetUserFromJWT(*req)
	userdb.UpdateUserSettings(username, settings)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func GetUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	username, _ := util.GetUserFromJWT(*req)

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userdb.GetUserSettings(username))
}

func EditUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var loginData vars.UserData
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)

	if !userdb.IsUserExists(loginData.Login) {
		json.NewEncoder(w).Encode(map[string]string{"error": "No such user"})
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func DeleteContainerLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var containerItem struct {
		Host    string `json:"host"`
		Service string `json:"service"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&containerItem)

	go containerdb.DeleteContainer(containerItem.Host, containerItem.Service, false)
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func DeleteDockerLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": util.DeleteDockerLogs(logItem.Host, logItem.Service)})
}

func DeleteContainer(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)

	if (logItem.Host == "" || logItem.Host == util.GetHost()) && strings.Contains(logItem.Service, "onlogs") {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Can't delete myself!"})
	}

	go containerdb.DeleteContainer(logItem.Host, logItem.Service, true)
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func DeleteUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var loginData struct {
		Login string `json:"login"`
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)
	if loginData.Login == "admin" {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Can't delete admin"})
		return
	}

	err := userdb.DeleteUser(loginData.Login, loginData.Login)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}
