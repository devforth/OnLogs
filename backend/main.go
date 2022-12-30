package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/joho/godotenv"
)

func main() {
	// os.RemoveAll("leveldb")
	godotenv.Load(".env")

	if os.Getenv("CLIENT") != "" {
		util.SendInitRequest()
		streamer.StreamLogs()
	}

	go streamer.StreamLogs()
	util.CreateInitUser()

	http.HandleFunc("/", routes.Frontend)
	http.HandleFunc("/api/v1/checkCookie", routes.CheckCookie)
	http.HandleFunc("/api/v1/addHost", routes.AddHost)
	http.HandleFunc("/api/v1/addLogLine", routes.AddLogLine)
	http.HandleFunc("/api/v1/createUser", routes.CreateUser)
	http.HandleFunc("/api/v1/getHosts", routes.GetHosts)
	http.HandleFunc("/api/v1/getSizeByService", routes.GetSizeByService)
	http.HandleFunc("/api/v1/getLogs", routes.GetLogs)
	http.HandleFunc("/api/v1/getUsers", routes.GetUsers)
	http.HandleFunc("/api/v1/getLogsStream", routes.GetLogsStream)
	http.HandleFunc("/api/v1/login", routes.Login)
	http.HandleFunc("/api/v1/logout", routes.Logout)
	http.HandleFunc("/api/v1/editUser", routes.EditUser)
	http.HandleFunc("/api/v1/deleteUser", routes.DeleteUser)

	fmt.Println("ONLOGS: ", http.ListenAndServe(":"+string(os.Getenv("PORT")), nil))
}
