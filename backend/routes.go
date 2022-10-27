package main

import (
	"encoding/json"
	"net/http"
	"os"
)

type Resp struct {
	Host     string   `json:"host"`
	Services []string `json:"services"`
}

func routeGetHost(w http.ResponseWriter, req *http.Request) {
	host, _ := os.Hostname()
	to_return := &Resp{Host: host, Services: getContainersList()}
	e, _ := json.Marshal(to_return)
	w.Write(e)
}

func routeGetContainerLogs(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(getContainerLogs(req.URL.Query().Get("id")))
}
