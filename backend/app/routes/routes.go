package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	daemon "github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
)

type Resp struct {
	Host     string   `json:"host"`
	Services []string `json:"services"`
}

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	host, _ := os.Hostname()
	to_return := &Resp{Host: host, Services: daemon.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

// func RouteGetContainerLogs(w http.ResponseWriter, req *http.Request) {
// 	params := req.URL.Query()
// 	json.NewEncoder(w).Encode(daemon.GetAllContainerLogs(params.Get("id")))
// }

func RouteGetLogs(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	limit, _ := strconv.Atoi(params.Get("limit"))
	offset, _ := strconv.Atoi(params.Get("offset"))
	json.NewEncoder(w).Encode(db.GetLogs(params.Get("id"), params.Get("search"), limit, offset))
}
