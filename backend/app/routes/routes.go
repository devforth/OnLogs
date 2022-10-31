package routes

import (
	"encoding/json"
	"net/http"
	"os"

	daemon "github.com/devforth/OnLogs/app/daemon"
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

func RouteGetContainerLogs(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	json.NewEncoder(w).Encode(daemon.GetAllContainerLogs(params.Get("id")))
}
