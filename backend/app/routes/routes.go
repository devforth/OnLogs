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
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}

func listener(conn *websocket.Conn) {
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(string(message))

		if err := conn.WriteMessage(messageType, message); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	enableCors(&w)
	host, _ := os.Hostname()
	to_return := &vars.HostsList{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func RouteGetLogsStream(w http.ResponseWriter, req *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Client Connected")

	container := req.URL.Query().Get("id")
	if strings.Compare(container, "") == 0 {
		return
	}
	vars.Connections[container] = append(vars.Connections[container], *ws)
	// err = ws.WriteMessage(1, []byte("Hi "+container))

	listener(ws)
}

func RouteGetLogs(w http.ResponseWriter, req *http.Request) {
	enableCors(&w)
	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	json.NewEncoder(w).Encode(srchx_db.GetLogs(params.Get("id"), params.Get("search"), limit, offset))
}

func RouteLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var loginData vars.UserData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)

		isCorrect := db.CheckUserPassword(loginData.Login, loginData.Password)
		if !isCorrect {
			json.NewEncoder(w).Encode(map[string]string{"error": "Wrong login or password!"})
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "onlogs-cookie",
			Value:   util.CreateJWT(loginData.Login),
			Expires: time.Now().AddDate(0, 0, 2),
			MaxAge:  int(time.Now().AddDate(0, 0, 2).Unix()),
		})
	}
}

func RouteCreateUser(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var loginData vars.UserData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)

		err := db.CreateUser(loginData.Login, loginData.Password)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		http.SetCookie(w, &http.Cookie{
			Name:    "onlogs-cookie",
			Value:   util.CreateJWT(loginData.Login),
			Expires: time.Now().AddDate(0, 0, 2),
			MaxAge:  int(time.Now().AddDate(0, 0, 2).Unix()),
		})
	}
}

func RouteDeleteUser(w http.ResponseWriter, req *http.Request) {
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
