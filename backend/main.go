package main

import (
	"fmt"
	"net/http"

	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/joho/godotenv"
)

func main() {
	util.RemoveOldFiles()
	go util.StartLogDumpGarbageCollector()
	godotenv.Load(".env")
	util.CreateInitUser()
	go streamer.StreamLogs()

	http.HandleFunc("/", routes.RouteFrontend)

	http.HandleFunc("/api/v1/checkCookie", routes.RouteCheckCookie)
	http.HandleFunc("/api/v1/getLogsStream", routes.RouteGetLogsStream)
	http.HandleFunc("/api/v1/getHost", routes.RouteGetHost)
	http.HandleFunc("/api/v1/getLogs", routes.RouteGetLogs)
	http.HandleFunc("/api/v1/login", routes.RouteLogin)
	http.HandleFunc("/api/v1/logout", routes.RouteLogout)
	http.HandleFunc("/api/v1/createUser", routes.RouteCreateUser)
	http.HandleFunc("/api/v1/getUsers", routes.RouteGetUsers)
	http.HandleFunc("/api/v1/editUser", routes.RouteEditUser)
	http.HandleFunc("/api/v1/deleteUser", routes.RouteDeleteUser)

	err := http.ListenAndServe(":2874", nil)
	fmt.Println("ONLOGS: ", err)
}
