package main

import (
	"net/http"
	"os"

	routes "github.com/devforth/OnLogs/app/routes"
	util "github.com/devforth/OnLogs/app/util"
)

func main() {
	os.RemoveAll("leveldb")
	go util.StreamLogs()

	http.HandleFunc("/api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("/api/v1/getLogs", routes.RouteGetLogs)

	http.ListenAndServe(":2874", nil)
}
