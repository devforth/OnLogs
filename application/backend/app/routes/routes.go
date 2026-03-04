package routes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/userdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
)

type RouteController struct {
    DockerService *docker.DockerService
    DaemonService *daemon.DaemonService
}

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
	if os.Getenv("DISABLE_AUTH") == "true" {
		return true
	}

	username, err := util.GetUserFromJWT(*req)
	if username != os.Getenv("ADMIN_USERNAME") {
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
	if os.Getenv("DISABLE_AUTH") == "true" {
		return true
	}

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

func (h *RouteController)Frontend(w http.ResponseWriter, req *http.Request) {
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
		fileName = "index.html"
	}
	if err != nil {
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	content, _ := io.ReadAll(file)

	if fileName == "index.html" {
		var disableAuth []byte
		if os.Getenv("DISABLE_AUTH") == "true" {
			disableAuth = []byte("true")
		} else {
			disableAuth = []byte("false")
		}

		content = bytes.Replace(content, []byte("$DISABLE_AUTH$"), disableAuth, 1)
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(fileName)))
	http.ServeContent(w, req, fileName, stat.ModTime(), bytes.NewReader(content))
}

func (h *RouteController)CheckCookie(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController)AddLogLine(w http.ResponseWriter, req *http.Request) {
	var logItem struct {
		Token     string
		Host      string
		Container string
		LogLine   []string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)

	if !db.IsTokenExists(logItem.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if vars.Counters_For_Hosts_Last_30_Min[logItem.Host] == nil {
		go statistics.RunStatisticForContainer(logItem.Host, logItem.Container)
	}
	err := containerdb.PutLogMessage(util.GetDB(logItem.Host, logItem.Container, "logs"), logItem.Host, logItem.Container, logItem.LogLine)
	if err != nil {
		defer w.WriteHeader(http.StatusInternalServerError)
		panic(err)
	}

	to_send, _ := json.Marshal([]string{logItem.LogLine[0], logItem.LogLine[1]})
	conns := vars.Connections[logItem.Host+"/"+logItem.Container]
	for i := range conns {
		c := conns[i]
		c.WriteMessage(websocket.TextMessage, to_send)
	}
}

func (h *RouteController)AddHost(w http.ResponseWriter, req *http.Request) {
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
	// fmt.Println("New host added: " + addReq.Hostname)  need to create separate route for SendUpdate func
	for _, container := range addReq.Services {
		os.MkdirAll("leveldb/hosts/"+addReq.Hostname+"/containers/"+container, 0700)
	}
}

func (h *RouteController)ChangeFavourite(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)GetSecret(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": db.CreateOnLogsToken()})
}

func (h *RouteController)GetChartData(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController) GetHosts(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	type HostsList struct {
		Host     string                   `json:"host"`
		Services []map[string]interface{} `json:"services"`
	}

	var to_return []HostsList
	ctx := req.Context()
	activeContainers := h.DaemonService.GetContainersList(ctx)

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

func (h *RouteController)GetSizeByAll(w http.ResponseWriter, req *http.Request) {
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
func (h *RouteController)GetSizeByService(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)GetDockerSize(w http.ResponseWriter, req *http.Request) {
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

	containerID := util.GetDockerContainerID(params.Get("host"), params.Get("service"))
	info, _ := os.Stat("/var/lib/docker/containers/" + containerID + "/" + containerID + "-json.log")

	size := float64(info.Size()) / (1024.0 * 1024.0)
	if size < 0.1 && size != 0.0 {
		size = 0.1
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", size)}) // MiB
}

func (h *RouteController)GetStats(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)GetStorageData(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var data struct {
		Host string `json:"host"`
	}

	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&data)

	w.Header().Add("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid data!"})
		return
	}

	// TODO make for different hosts
	if data.Host != util.GetHost() {
		json.NewEncoder(w).Encode(map[string]string{"error": "For now working only for main host.\nAsked host: " + data.Host + "\nIt's ok to see this message, all works fine."})
		return
	}
	json.NewEncoder(w).Encode(util.GetStorageData())
}

func (h *RouteController)GetPrevLogs(w http.ResponseWriter, req *http.Request) {
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
	json.NewEncoder(w).Encode(containerdb.GetLogs(true, false, params.Get("host"), params.Get("id"), params.Get("search"), limit, params.Get("startWith"), caseSensetive, nil))
}

func (h *RouteController)GetLogs(w http.ResponseWriter, req *http.Request) {
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

	status := params.Get("status")
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}

	json.NewEncoder(w).Encode(containerdb.GetLogs(
		false, false, params.Get("host"), params.Get("id"), params.Get("search"),
		limit, params.Get("startWith"), caseSensetive, statusPtr,
	))
}

func (h *RouteController)GetLogWithPrev(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	w.Header().Add("Content-Type", "application/json")
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	json.NewEncoder(w).Encode(containerdb.GetLogs(false, true, params.Get("host"), params.Get("id"), "", limit, params.Get("startWith"), false, nil))
}

// TODO return {"error": "Invalid host!"} when host is not exists
func (h *RouteController)GetLogsStream(w http.ResponseWriter, req *http.Request) {
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
	vars.Connections[container] = append(vars.Connections[container], ws)
}

func (h *RouteController)Login(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)Logout(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)CreateUser(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)GetUsers(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	users := userdb.GetUsers()
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users, "error": nil})
}

func (h *RouteController)UpdateUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var settings map[string]interface{}
	body, _ := io.ReadAll(req.Body)
	json.Unmarshal(body, &settings)
	username, _ := util.GetUserFromJWT(*req)
	userdb.UpdateUserSettings(username, settings)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController)GetUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	username, _ := util.GetUserFromJWT(*req)

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userdb.GetUserSettings(username))
}

func (h *RouteController)EditHostname(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var data struct {
		Host string
		Name string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&data)

	if data.Name != "" {
		if data.Host != util.GetHost() {
			// TODO ask for command
		} else {
			os.WriteFile("/etc/hosntame", []byte(data.Name), 0644)
		}
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController)EditUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var loginData vars.UserData
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)

	if loginData.Login == os.Getenv("ADMIN_USERNAME") {
		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Can't edit admin. Use env variables to change admin username and password"})
		return
	}

	if !userdb.IsUserExists(loginData.Login) {
		json.NewEncoder(w).Encode(map[string]string{"error": "No such user"})
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController)DeleteContainerLogs(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)DeleteDockerLogs(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController)AskForDelete(w http.ResponseWriter, req *http.Request) {
	var logItem struct {
		Hostname string
		Token    string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)

	if !db.IsTokenExists(logItem.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	to_delete := []string{}
	if len(vars.ToDelete[logItem.Hostname]) != 0 {
		to_delete = vars.ToDelete[logItem.Hostname]
		vars.ToDelete[logItem.Hostname] = []string{}
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"Services": to_delete})
}

func (h *RouteController)DeleteContainer(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&logItem)

	if logItem.Host == "" || logItem.Host == util.GetHost() {
		dockerContainerID := util.GetDockerContainerID(logItem.Host, logItem.Service)
		dockerImage, _ := h.DockerService.GetContainerImageNameByContainerID(req.Context(), dockerContainerID)
		if strings.Contains(dockerImage, "devforth/onlogs") {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Can't delete logs of OnLogs container!"})
			return
		}
	}

	go containerdb.DeleteContainer(logItem.Host, logItem.Service, true)
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController)DeleteUser(w http.ResponseWriter, req *http.Request) {
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
	if loginData.Login == os.Getenv("ADMIN_USERNAME") {
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
