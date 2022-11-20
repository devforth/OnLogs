package main

import (
	"net/http"

	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/joho/godotenv"
)

func main() {
	util.RemoveOldFiles()
	godotenv.Load(".env")
	util.CreateInitUser()
	go streamer.StreamLogs()

	http.HandleFunc("/api/v1/getLogsStream", routes.RouteGetLogsStream)
	http.HandleFunc("/api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("/api/v1/getLogs", routes.RouteGetLogs)
	http.HandleFunc("/api/v1/login", routes.RouteLogin)
	http.HandleFunc("/api/v1/createUser", routes.RouteCreateUser)
	http.HandleFunc("/api/v1/deleteUser", routes.RouteDeleteUser)

	http.ListenAndServe(":2874", nil)
}
