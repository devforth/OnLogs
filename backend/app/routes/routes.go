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

	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
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

func verifyOnlogsToken(token string) bool {
	if token == os.Getenv("ONLOGS_TOKEN") {
		return true
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
	requestedPath := req.URL.String()
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
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}

func AddHost(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) {
		return
	}

	if req.Method != "POST" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var addReq struct {
		Hostname string
		Token    string
	}
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&addReq)

	if !verifyOnlogsToken(addReq.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	fileContent, err := ioutil.ReadFile("leveldb/hosts/hostsList")
	fmt.Println(fileContent)
	if err != nil {
		os.MkdirAll("leveldb/hosts", 0700)
		os.WriteFile("leveldb/hosts/hostsList", []byte(req.RemoteAddr+"\n"), 0777)
		return
	} else {
		if util.Contains(req.RemoteAddr, strings.Split(string(fileContent), "\n")) {
			return
		}
	}

	os.WriteFile("leveldb/hosts/hostsList", []byte(string(fileContent)+req.RemoteAddr+"\n"), 0777)
}

func GetHosts(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	fileContent, err := ioutil.ReadFile("leveldb/hosts/hostsList")
	if err != nil {
		util.CreateInitHost()
		fileContent, _ = ioutil.ReadFile("leveldb/hosts/hostsList")
	}
	hosts := strings.Split(string(fileContent), "\n")
	var to_return []vars.HostsList

	for _, host := range hosts {
		resp, err := http.Get(host + "/api/v1/getHost")
		if err == nil {
			var result vars.HostsList
			json.NewDecoder(resp.Body).Decode(&result)
			to_return = append(to_return, result)
		} else {
			fmt.Println("ERROR: ", err)
		}
	}

	host := util.GetHost()
	to_return = append(to_return, vars.HostsList{Host: host, Services: vars.All_Containers})
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func GetLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	caseSensetive, err := strconv.ParseBool(params.Get("caseSens"))
	if err != nil {
		caseSensetive = false
	}
	json.NewEncoder(w).Encode(db.GetLogs(params.Get("id"), params.Get("search"), limit, offset, params.Get("startWith"), caseSensetive))
}

func GetLogsStream(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
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

	container := req.URL.Query().Get("id")
	if strings.Compare(container, "") == 0 {
		return
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

	isCorrect := db.CheckUserPassword(loginData.Login, loginData.Password)
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

	err := db.CreateUser(loginData.Login, loginData.Password)
	if err == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func GetUsers(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	users := db.GetUsers()
	json.NewEncoder(w).Encode(map[string][]string{"users": users})
}

func EditUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyAdminUser(&w, req) {
		return
	}

	var loginData vars.UserData
	decoder := json.NewDecoder(req.Body)
	decoder.Decode(&loginData)

	if !db.IsUserExists(loginData.Login) {
		json.NewEncoder(w).Encode(map[string]string{"error": "No such user"})
		return
	}

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
		json.NewEncoder(w).Encode(map[string]string{"error": "Can't delete admin"})
		return
	}

	err := db.DeleteUser(loginData.Login, loginData.Login)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"error": nil})
}
