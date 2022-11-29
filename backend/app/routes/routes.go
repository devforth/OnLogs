package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/srchx_db"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func verifyUser(w *http.ResponseWriter, req *http.Request) bool {
	_, err := util.GetUserFromJWT(*req)
	if err != nil {
		(*w).WriteHeader(http.StatusUnauthorized)
		(*w).Write([]byte(err.Error()))
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

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	host, _ := os.Hostname()
	to_return := &vars.HostsList{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func RouteGetLogs(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	json.NewEncoder(w).Encode(srchx_db.GetLogs(params.Get("id"), params.Get("search"), limit, offset))
}

func RouteGetLogsStream(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
		return
	}

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

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

func RouteLogin(w http.ResponseWriter, req *http.Request) {
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

func RouteCreateUser(w http.ResponseWriter, req *http.Request) {
	if verifyRequest(&w, req) || !verifyUser(&w, req) {
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
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	}
}

func RouteDeleteUser(w http.ResponseWriter, req *http.Request) {
	enableCors(&w)
	if req.Method == "POST" {
		var loginData vars.UserData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)

		err := db.DeleteUser(loginData.Login, loginData.Password)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
	}
}
