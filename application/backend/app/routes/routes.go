package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/groups"
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

func isAllowedOrigin(req *http.Request) bool {
	origin := req.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if os.Getenv("ENV_NAME") == "local" && parsed.Host == "localhost:5173" {
		return true
	}
	return strings.EqualFold(parsed.Host, req.Host)
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

const (
	maxRequestBody  = 1 << 20
	sessionLifetime = 48 * time.Hour
)

func isTLS(req *http.Request) bool {
	return req.TLS != nil || strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
}

func decodeBody(w http.ResponseWriter, req *http.Request, target interface{}) bool {
	req.Body = http.MaxBytesReader(w, req.Body, maxRequestBody)
	if err := json.NewDecoder(req.Body).Decode(target); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
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

func (h *RouteController) Frontend(w http.ResponseWriter, req *http.Request) {
	// http.Dir sanitises the name it is given but never its own root.
	dir := http.Dir("dist")

	fileName := strings.TrimPrefix(strings.TrimPrefix(req.URL.Path, os.Getenv("ONLOGS_PATH_PREFIX")), "/")
	if fileName == "" || strings.HasSuffix(fileName, "/") {
		fileName = "index.html"
	}

	file, err := dir.Open(fileName)
	if err != nil {
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

func (h *RouteController) CheckCookie(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) AddLogLine(w http.ResponseWriter, req *http.Request) {
	var logItem struct {
		Token     string
		Host      string
		Container string
		LogLine   []string
	}
	if !decodeBody(w, req, &logItem) {
		return
	}

	if !db.IsTokenExists(logItem.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !util.IsSafeName(logItem.Host) || !util.IsSafeName(logItem.Container) || len(logItem.LogLine) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	statistics.EnsureWorker(context.Background(), logItem.Host, logItem.Container)
	err := containerdb.PutLogMessage(util.GetDB(logItem.Host, logItem.Container, "logs"), logItem.Host, logItem.Container, logItem.LogLine)
	if err != nil {
		defer w.WriteHeader(http.StatusInternalServerError)
		panic(err)
	}

	to_send, _ := json.Marshal([]string{logItem.LogLine[0], logItem.LogLine[1]})
	vars.Broadcast(logItem.Host+"/"+logItem.Container, websocket.TextMessage, to_send)
}

func (h *RouteController) AddHost(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var addReq struct {
		Hostname string
		Token    string
		Services []string
	}
	if !decodeBody(w, req, &addReq) {
		return
	}

	if !db.IsTokenExists(addReq.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if !util.IsSafeName(addReq.Hostname) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for _, container := range addReq.Services {
		if !util.IsSafeName(container) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	vars.SetAgentContainers(addReq.Hostname, addReq.Services)
	// fmt.Println("New host added: " + addReq.Hostname)  need to create separate route for SendUpdate func
	for _, container := range addReq.Services {
		os.MkdirAll("leveldb/hosts/"+addReq.Hostname+"/containers/"+container, 0700)
	}
}

func (h *RouteController) ChangeFavourite(w http.ResponseWriter, req *http.Request) {
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
	if !decodeBody(w, req, &container) {
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	key := favouriteKey(username, container.Host, container.Service)

	isAlreadyFavourite, err := vars.FavsDB.Has(key, nil)
	if err == nil {
		if isAlreadyFavourite {
			err = vars.FavsDB.Delete(key, nil)
		} else {
			err = vars.FavsDB.Put(key, nil, nil)
		}
	}

	w.Header().Add("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func groupError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (h *RouteController) GetGroups(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	list, err := groups.List(username)

	w.Header().Add("Content-Type", "application/json")
	if err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	json.NewEncoder(w).Encode(list)
}

func (h *RouteController) CreateGroup(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var body struct {
		Name    string          `json:"name"`
		Members []groups.Member `json:"members"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := groups.ValidateGroupName(body.Name); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateMembers(body.Members); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	existing, err := groups.List(username)
	if err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, group := range existing {
		if group.Name == body.Name {
			groupError(w, http.StatusConflict, "Group \""+body.Name+"\" already exists")
			return
		}
	}
	// An authenticated user must not be able to write unbounded data to LevelDB.
	if len(existing) >= groups.MaxGroupsPerUser {
		groupError(w, http.StatusBadRequest,
			fmt.Sprintf("You can not have more than %d groups", groups.MaxGroupsPerUser))
		return
	}

	if err := groups.Store(username, body.Name, body.Members); err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) UpdateGroup(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var body struct {
		Name    string          `json:"name"`
		NewName string          `json:"newName"`
		Members []groups.Member `json:"members"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	newName := body.NewName
	if newName == "" {
		newName = body.Name
	}

	w.Header().Add("Content-Type", "application/json")
	if err := groups.ValidateGroupName(body.Name); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateGroupName(newName); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateMembers(body.Members); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	_, found, err := groups.Load(username, body.Name)
	if err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		groupError(w, http.StatusNotFound, "No such group")
		return
	}

	if newName != body.Name {
		if _, taken, _ := groups.Load(username, newName); taken {
			groupError(w, http.StatusConflict, "Group \""+newName+"\" already exists")
			return
		}
	}

	if err := groups.Store(username, newName, body.Members); err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if newName != body.Name {
		if err := groups.Delete(username, body.Name); err != nil {
			groupError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) DeleteGroup(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := groups.ValidateGroupName(body.Name); err != nil {
		groupError(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	// Deleting what is not there is a success, because the UI may double-fire.
	if err := groups.Delete(username, body.Name); err != nil {
		groupError(w, http.StatusInternalServerError, err.Error())
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) GetSecret(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": db.CreateOnLogsToken()})
}

func (h *RouteController) GetChartData(w http.ResponseWriter, req *http.Request) {
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
	if !decodeBody(w, req, &data) {
		return
	}

	if !util.Contains(data.Unit, []string{"hour", "day", "month"}) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid data!"})
		return
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

	to_return := []HostsList{}
	viewer, _ := util.GetUserFromJWT(*req)
	ctx := req.Context()
	activeContainers := h.DaemonService.GetContainersList(ctx)

	hosts, _ := os.ReadDir("leveldb/hosts/")
	for _, host := range hosts {
		containers, _ := os.ReadDir("leveldb/hosts/" + host.Name() + "/containers")
		allContainers := []map[string]interface{}{}
		for _, container := range containers {
			isFavorite, _ := vars.FavsDB.Has(favouriteKey(viewer, host.Name(), container.Name()), nil)
			if util.Contains(container.Name(), activeContainers) || util.Contains(container.Name(), vars.AgentContainers(host.Name())) {
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

func (h *RouteController) GetSizeByAll(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	_, totalBytes := util.LogsSizes()
	totalSize := float64(totalBytes) / (1024.0 * 1024.0)

	if totalSize < 0.1 && totalSize != 0.0 {
		totalSize = 0.1
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", totalSize)}) // MiB
}

// TODO need to return 0.0 when there is no logs for container in db
func (h *RouteController) GetSizeByService(w http.ResponseWriter, req *http.Request) {
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

func (h *RouteController) GetDockerSize(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	if params.Get("service") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if params.Get("host") == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Host is not mentioned!"})
		return
	}
	w.Header().Add("Content-Type", "application/json")

	// A container with no docker json log has size 0, not a nil dereference.
	var size float64
	containerID := util.GetDockerContainerID(params.Get("host"), params.Get("service"))
	if containerID != "" {
		if info, err := os.Stat("/var/lib/docker/containers/" + containerID + "/" + containerID + "-json.log"); err == nil && info != nil {
			size = float64(info.Size()) / (1024.0 * 1024.0)
		}
	}

	if size < 0.1 && size != 0.0 {
		size = 0.1
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", size)}) // MiB
}

func (h *RouteController) GetStats(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var data struct {
		Host    string `json:"host"`
		Service string `json:"service"`
		Value   int    `json:"period"` // 1 = 30min, 2 = 1hr, 48 = 1d
	}

	if !decodeBody(w, req, &data) {
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statistics.GetStatisticsByService(data.Host, data.Service, data.Value))
}

func (h *RouteController) GetStorageData(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var data struct {
		Host string `json:"host"`
	}

	if !decodeBody(w, req, &data) {
		return
	}
	w.Header().Add("Content-Type", "application/json")

	// TODO make for different hosts
	if data.Host != util.GetHost() {
		json.NewEncoder(w).Encode(map[string]string{"error": "For now working only for main host.\nAsked host: " + data.Host + "\nIt's ok to see this message, all works fine."})
		return
	}
	json.NewEncoder(w).Encode(util.GetStorageData())
}

// Favourites are per user, like the user settings stored next to them.
func favouriteKey(username string, host string, service string) []byte {
	return []byte(username + "\x00" + host + "/" + service)
}

func statusFilter(params url.Values) *string {
	status := params.Get("status")
	if status == "" {
		return nil
	}
	return &status
}

func (h *RouteController) GetPrevLogs(w http.ResponseWriter, req *http.Request) {
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
	json.NewEncoder(w).Encode(containerdb.GetLogs(true, false, params.Get("host"), params.Get("id"), params.Get("search"), limit, params.Get("startWith"), caseSensetive, statusFilter(params)))
}

func (h *RouteController) GetLogs(w http.ResponseWriter, req *http.Request) {
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

	json.NewEncoder(w).Encode(containerdb.GetLogs(
		false, false, params.Get("host"), params.Get("id"), params.Get("search"),
		limit, params.Get("startWith"), caseSensetive, statusFilter(params),
	))
}

func (h *RouteController) GetLogWithPrev(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	w.Header().Add("Content-Type", "application/json")
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	json.NewEncoder(w).Encode(containerdb.GetLogs(false, true, params.Get("host"), params.Get("id"), "", limit, params.Get("startWith"), false, statusFilter(params)))
}

// TODO return {"error": "Invalid host!"} when host is not exists
func (h *RouteController) GetLogsStream(w http.ResponseWriter, req *http.Request) {
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
		CheckOrigin:     isAllowedOrigin,
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		// gorilla returns (nil, err); appending here seeds a nil conn that a
		// background goroutine later dereferences.
		fmt.Println(err)
		return
	}
	vars.AddConnection(container, ws)
}

func (h *RouteController) Login(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var loginData vars.UserData
	if !decodeBody(w, req, &loginData) {
		return
	}

	// Keyed on the address and on the (address, login) pair -- never on the login
	// alone, or anyone could lock any account out by guessing at it.
	addr := clientAddr(req)
	ipKey := "ip:" + addr
	pairKey := "pair:" + loginKey(addr, loginData.Login)
	if !loginLimiter.allow(ipKey, pairKey) {
		vars.LoginBlocked.Add(1)
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"error": "Too many failed login attempts. Try again later."})
		return
	}

	isCorrect := userdb.CheckUserPassword(loginData.Login, loginData.Password)
	if !isCorrect {
		// Once per request, not once per key: fail() takes two.
		vars.LoginFailures.Add(1)
		loginLimiter.fail(ipKey, pairKey)
		json.NewEncoder(w).Encode(map[string]string{"error": "Wrong login or password!"})
		return
	}
	loginLimiter.succeed(ipKey, pairKey)

	http.SetCookie(w, &http.Cookie{
		Name:     "onlogs-cookie",
		Value:    util.CreateJWT(loginData.Login),
		Expires:  time.Now().AddDate(0, 0, 2),
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   isTLS(req),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) Logout(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "onlogs-cookie",
		Value:    "toDelete",
		Expires:  time.Now().AddDate(-5, -5, -5),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isTLS(req),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) CreateUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var loginData vars.UserData
	if !decodeBody(w, req, &loginData) {
		return
	}

	err := userdb.CreateUser(loginData.Login, loginData.Password)
	if err == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
		return
	}
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func (h *RouteController) GetUsers(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	users := userdb.GetUsers()
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"users": users, "error": nil})
}

func (h *RouteController) UpdateUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var settings map[string]interface{}
	if !decodeBody(w, req, &settings) {
		return
	}
	username, _ := util.GetUserFromJWT(*req)
	w.Header().Add("Content-Type", "application/json")
	if err := userdb.UpdateUserSettings(username, settings); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) GetUserSettings(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}
	username, _ := util.GetUserFromJWT(*req)

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userdb.GetUserSettings(username))
}

func (h *RouteController) EditUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var loginData vars.UserData
	if !decodeBody(w, req, &loginData) {
		return
	}

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

	if loginData.Password == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "Password can not be empty"})
		return
	}

	if err := userdb.EditUser(loginData.Login, loginData.Password); err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) DeleteContainerLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var containerItem struct {
		Host    string `json:"host"`
		Service string `json:"service"`
	}
	if !decodeBody(w, req, &containerItem) {
		return
	}

	go containerdb.DeleteContainer(containerItem.Host, containerItem.Service, false)
	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func (h *RouteController) DeleteDockerLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	if !decodeBody(w, req, &logItem) {
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": util.DeleteDockerLogs(logItem.Host, logItem.Service)})
}

func (h *RouteController) AskForDelete(w http.ResponseWriter, req *http.Request) {
	var logItem struct {
		Hostname string
		Token    string
	}
	if !decodeBody(w, req, &logItem) {
		return
	}

	if !db.IsTokenExists(logItem.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	to_delete := vars.TakeQueuedDeletes(logItem.Hostname)

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"Services": to_delete})
}

func (h *RouteController) DeleteContainer(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	if !decodeBody(w, req, &logItem) {
		return
	}

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

func (h *RouteController) DeleteUser(w http.ResponseWriter, req *http.Request) {
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
	if !decodeBody(w, req, &loginData) {
		return
	}
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
