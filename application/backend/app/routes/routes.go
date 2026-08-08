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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/groups"
	"github.com/devforth/OnLogs/app/hostalias"
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

func enableCors(w http.ResponseWriter) {
	var origin string
	if os.Getenv("ENV_NAME") == "local" {
		origin = "http://localhost:5173"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

func verifyAdminUser(w http.ResponseWriter, req *http.Request) bool {
	if os.Getenv("DISABLE_AUTH") == "true" {
		return true
	}

	username, err := util.GetUserFromJWT(*req)
	if username != os.Getenv("ADMIN_USERNAME") {
		fail(w, http.StatusForbidden, "Only admin can perform this request")
		return false
	}

	if err != nil {
		fail(w, http.StatusUnauthorized, err.Error())
		return false
	}
	return true
}

func verifyUser(w http.ResponseWriter, req *http.Request) bool {
	if os.Getenv("DISABLE_AUTH") == "true" {
		return true
	}

	if _, err := util.GetUserFromJWT(*req); err != nil {
		fail(w, http.StatusUnauthorized, err.Error())
		return false
	}
	return true
}

type authLevel int

const (
	authNone authLevel = iota
	authUser
	authAdmin
)

// False means the request must not reach the handler body: a CORS preflight, a
// rejected credential, or the wrong method. An empty method accepts any.
func guard(w http.ResponseWriter, req *http.Request, level authLevel, method string) bool {
	enableCors(w)
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return false
	}

	switch level {
	case authUser:
		if !verifyUser(w, req) {
			return false
		}
	case authAdmin:
		if !verifyAdminUser(w, req) {
			return false
		}
	}

	if method != "" && req.Method != method {
		w.WriteHeader(http.StatusNotFound)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	json.NewEncoder(w).Encode(payload)
}

func ok(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{"error": nil})
}

func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
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
		fail(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
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
	if !guard(w, req, authUser, "") {
		return
	}
	ok(w)
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
	if !guard(w, req, authUser, "POST") {
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

	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w)
}

func (h *RouteController) GetGroups(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	list, err := groups.List(username)

	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RouteController) CreateGroup(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "POST") {
		return
	}

	var body struct {
		Name    string          `json:"name"`
		Members []groups.Member `json:"members"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	if err := groups.ValidateGroupName(body.Name); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateMembers(body.Members); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	existing, err := groups.List(username)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, group := range existing {
		if group.Name == body.Name {
			fail(w, http.StatusConflict, "Group \""+body.Name+"\" already exists")
			return
		}
	}
	// An authenticated user must not be able to write unbounded data to LevelDB.
	if len(existing) >= groups.MaxGroupsPerUser {
		fail(w, http.StatusBadRequest,
			fmt.Sprintf("You can not have more than %d groups", groups.MaxGroupsPerUser))
		return
	}

	if err := groups.Store(username, body.Name, body.Members); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w)
}

func (h *RouteController) UpdateGroup(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "POST") {
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

	if err := groups.ValidateGroupName(body.Name); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateGroupName(newName); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := groups.ValidateMembers(body.Members); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	_, found, err := groups.Load(username, body.Name)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		fail(w, http.StatusNotFound, "No such group")
		return
	}

	if newName != body.Name {
		if _, taken, _ := groups.Load(username, newName); taken {
			fail(w, http.StatusConflict, "Group \""+newName+"\" already exists")
			return
		}
	}

	if err := groups.Store(username, newName, body.Members); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if newName != body.Name {
		if err := groups.Delete(username, body.Name); err != nil {
			fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	ok(w)
}

func (h *RouteController) DeleteGroup(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "POST") {
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	if err := groups.ValidateGroupName(body.Name); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	username, _ := util.GetUserFromJWT(*req)
	// Deleting what is not there is a success, because the UI may double-fire.
	if err := groups.Delete(username, body.Name); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w)
}

func (h *RouteController) GetHostAliases(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	aliases, err := hostalias.All()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, aliases)
}

func (h *RouteController) SetHostAlias(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "POST") {
		return
	}

	var body struct {
		Host  string `json:"host"`
		Alias string `json:"alias"`
	}
	if !decodeBody(w, req, &body) {
		return
	}

	if err := hostalias.Set(body.Host, body.Alias); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	ok(w)
}

func (h *RouteController) GetSecret(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": db.CreateOnLogsToken()})
}

func (h *RouteController) GetChartData(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "POST") {
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

	if !slices.Contains([]string{"hour", "day", "month"}, data.Unit) {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, http.StatusOK, map[string]interface{}{"error": "Invalid data!"})
		return
	}

	writeJSON(w, http.StatusOK, statistics.GetChartData(data.Host, data.Service, data.Unit, data.UnitsAmount))
}

func (h *RouteController) GetHosts(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
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
			if slices.Contains(activeContainers, container.Name()) || slices.Contains(vars.AgentContainers(host.Name()), container.Name()) {
				allContainers = append(allContainers, map[string]interface{}{"serviceName": container.Name(), "isDisabled": false, "isFavorite": isFavorite})
			} else {
				allContainers = append(allContainers, map[string]interface{}{"serviceName": container.Name(), "isDisabled": true, "isFavorite": isFavorite})
			}
		}
		to_return = append(to_return, HostsList{Host: host.Name(), Services: allContainers})
	}

	writeJSON(w, http.StatusOK, to_return)
}

func (h *RouteController) GetSizeByAll(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	_, totalBytes := util.LogsSizes()
	totalSize := float64(totalBytes) / (1024.0 * 1024.0)

	if totalSize < 0.1 && totalSize != 0.0 {
		totalSize = 0.1
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", totalSize)}) // MiB
}

// TODO need to return 0.0 when there is no logs for container in db
func (h *RouteController) GetSizeByService(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
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

	size := util.GetDirSize(params.Get("host"), params.Get("service"))
	if size < 0.1 && size != 0.0 {
		size = 0.1
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", size)}) // MiB
}

func (h *RouteController) GetDockerSize(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	params := req.URL.Query()
	if params.Get("service") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if params.Get("host") == "" {
		fail(w, http.StatusBadRequest, "Host is not mentioned!")
		return
	}

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
	writeJSON(w, http.StatusOK, map[string]interface{}{"sizeMiB": fmt.Sprintf("%.1f", size)}) // MiB
}

func (h *RouteController) GetStats(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
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
	writeJSON(w, http.StatusOK, statistics.GetStatisticsByService(data.Host, data.Service, data.Value))
}

func (h *RouteController) GetStorageData(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	var data struct {
		Host string `json:"host"`
	}

	if !decodeBody(w, req, &data) {
		return
	}

	// TODO make for different hosts
	if data.Host != util.GetHost() {
		fail(w, http.StatusOK, "For now working only for main host.\nAsked host: "+data.Host+"\nIt's ok to see this message, all works fine.")
		return
	}
	writeJSON(w, http.StatusOK, util.GetStorageData())
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
	if !guard(w, req, authUser, "") {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	caseSensetive, err := strconv.ParseBool(params.Get("caseSens"))
	if err != nil {
		caseSensetive = false
	}

	if params.Get("startWith") == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"error": "Need to specify \"startWith\"!"})
		return
	}

	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	writeJSON(w, http.StatusOK, containerdb.GetLogs(true, false, params.Get("host"), params.Get("id"), params.Get("search"), limit, params.Get("startWith"), caseSensetive, statusFilter(params)))
}

func (h *RouteController) GetLogs(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	caseSensetive, err := strconv.ParseBool(params.Get("caseSens"))
	if err != nil {
		caseSensetive = false
	}
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}

	writeJSON(w, http.StatusOK, containerdb.GetLogs(
		false, false, params.Get("host"), params.Get("id"), params.Get("search"),
		limit, params.Get("startWith"), caseSensetive, statusFilter(params),
	))
}

func (h *RouteController) GetLogWithPrev(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	if params.Get("host") == "" {
		panic("Host is not mentioned!")
	}
	writeJSON(w, http.StatusOK, containerdb.GetLogs(false, true, params.Get("host"), params.Get("id"), "", limit, params.Get("startWith"), false, statusFilter(params)))
}

// TODO return {"error": "Invalid host!"} when host is not exists
func (h *RouteController) GetLogsStream(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
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
	enableCors(w)
	if req.Method == http.MethodOptions {
		ok(w)
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
		fail(w, http.StatusTooManyRequests, "Too many failed login attempts. Try again later.")
		return
	}

	isCorrect := userdb.CheckUserPassword(loginData.Login, loginData.Password)
	if !isCorrect {
		// Once per request, not once per key: fail() takes two.
		vars.LoginFailures.Add(1)
		loginLimiter.fail(ipKey, pairKey)
		fail(w, http.StatusOK, "Wrong login or password!")
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
	ok(w)
}

func (h *RouteController) Logout(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
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
	ok(w)
}

func (h *RouteController) CreateUser(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "POST") {
		return
	}

	var loginData vars.UserData
	if !decodeBody(w, req, &loginData) {
		return
	}

	err := userdb.CreateUser(loginData.Login, loginData.Password)
	if err == nil {
		ok(w)
		return
	}
	fail(w, http.StatusOK, err.Error())
}

func (h *RouteController) GetUsers(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
		return
	}

	users := userdb.GetUsers()
	writeJSON(w, http.StatusOK, map[string]interface{}{"users": users, "error": nil})
}

func (h *RouteController) UpdateUserSettings(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}

	var settings map[string]interface{}
	if !decodeBody(w, req, &settings) {
		return
	}
	username, _ := util.GetUserFromJWT(*req)
	if err := userdb.UpdateUserSettings(username, settings); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w)
}

func (h *RouteController) GetUserSettings(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authUser, "") {
		return
	}
	username, _ := util.GetUserFromJWT(*req)

	writeJSON(w, http.StatusOK, userdb.GetUserSettings(username))
}

func (h *RouteController) EditUser(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
		return
	}

	var loginData vars.UserData
	if !decodeBody(w, req, &loginData) {
		return
	}

	if loginData.Login == os.Getenv("ADMIN_USERNAME") {
		fail(w, http.StatusOK, "Can't edit admin. Use env variables to change admin username and password")
		return
	}

	if !userdb.IsUserExists(loginData.Login) {
		fail(w, http.StatusOK, "No such user")
		return
	}

	if loginData.Password == "" {
		fail(w, http.StatusOK, "Password can not be empty")
		return
	}

	if err := userdb.EditUser(loginData.Login, loginData.Password); err != nil {
		fail(w, http.StatusOK, err.Error())
		return
	}

	ok(w)
}

func (h *RouteController) DeleteContainerLogs(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
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
	ok(w)
}

func (h *RouteController) DeleteDockerLogs(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
		return
	}

	var logItem struct {
		Host    string
		Service string
	}
	if !decodeBody(w, req, &logItem) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"error": util.DeleteDockerLogs(logItem.Host, logItem.Service)})
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

	writeJSON(w, http.StatusOK, map[string]interface{}{"Services": to_delete})
}

func (h *RouteController) DeleteContainer(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "") {
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
			writeJSON(w, http.StatusOK, map[string]interface{}{"error": "Can't delete logs of OnLogs container!"})
			return
		}
	}

	go containerdb.DeleteContainer(logItem.Host, logItem.Service, true)
	ok(w)
}

func (h *RouteController) DeleteUser(w http.ResponseWriter, req *http.Request) {
	if !guard(w, req, authAdmin, "POST") {
		return
	}

	var loginData struct {
		Login string `json:"login"`
	}
	if !decodeBody(w, req, &loginData) {
		return
	}
	if loginData.Login == os.Getenv("ADMIN_USERNAME") {
		fail(w, http.StatusOK, "Can't delete admin")
		return
	}

	err := userdb.DeleteUser(loginData.Login)
	if err != nil {
		fail(w, http.StatusOK, err.Error())
		return
	}
	ok(w)
}
