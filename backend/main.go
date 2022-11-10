package main

import (
	"net/http"
	"os"

	routes "github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
)

func main() {
	os.RemoveAll("leveldb")
	go streamer.StreamLogs()

	http.HandleFunc("/api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("/api/v1/getLogs", routes.RouteGetLogs)
	http.HandleFunc("/api/v1/login", routes.RouteLogin)
	http.HandleFunc("/api/v1/createUser", routes.RouteCreateUser)

	http.ListenAndServe(":2874", nil)
}
