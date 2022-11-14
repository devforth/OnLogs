package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/srchx_db"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	(*w).Header().Set("Access-Control-Allow-Methods", "*")
	(*w).Header().Set("Access-Control-Allow-Headers", "*")
}

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	enableCors(&w)
	host, _ := os.Hostname()
	to_return := &vars.HostsList{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
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
