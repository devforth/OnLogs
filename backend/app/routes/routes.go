package routes

import (
	"encoding/json"
	"net/http"
	"os"

	onlogs "github.com/devforth/OnLogs/app/requests"
)

type Resp struct {
	Host     string   `json:"host"`
	Services []string `json:"services"`
}

func RouteGetHost(w http.ResponseWriter, req *http.Request) {
	host, _ := os.Hostname()
	to_return := &Resp{Host: host, Services: onlogs.GetContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func RouteGetContainerLogs(w http.ResponseWriter, req *http.Request) {
	params := req.URL.Query()
	json.NewEncoder(w).Encode(onlogs.GetContainerLogs(params.Get("id"), 0, 0))
}
