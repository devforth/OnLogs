package main

import (
	"net/http"

	routes "github.com/devforth/OnLogs/app/routes"
	util "github.com/devforth/OnLogs/app/util"
)

func main() {
	util.StoreLogs() // store logs from all containers before getting started
	// fmt.Println("a")

	http.HandleFunc("/api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("/api/v1/getLogs", routes.RouteGetLogs)

	http.ListenAndServe(":2874", nil)
}
