package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	daemon "github.com/devforth/OnLogs/app/daemon"
	responses "github.com/devforth/OnLogs/app/responses"
	"github.com/devforth/OnLogs/app/srchx_db"
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
	to_return := &responses.HostsList{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func RouteGetLogs(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	json.NewEncoder(w).Encode(srchx_db.GetLogs(params.Get("id"), params.Get("search"), limit, offset))
}

func RouteLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		body := req.ParseForm()
		fmt.Println(body)
	}
}

func RouteCreateUser(w http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var loginData responses.LoginData
		decoder := json.NewDecoder(req.Body)
		decoder.Decode(&loginData)
	}
}
