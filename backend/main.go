package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load(".env")
	if os.Getenv("CLIENT") != "" {
		util.SendInitRequest()
		streamer.StreamLogs()
	}

	util.ReplacePrefixVariableForFrontend()
	go db.DeleteUnusedTokens()
	go streamer.StreamLogs()
	util.CreateInitUser()

	pathPrefix := os.Getenv("ONLOGS_PATH_PREFIX")
	if pathPrefix != "" {
		http.HandleFunc(pathPrefix+"/", routes.Frontend)
	} else {
		http.HandleFunc("/", routes.Frontend)
	}
	// http.HandleFunc("/", routes.Frontend)
	http.HandleFunc(pathPrefix+"/api/v1/checkCookie", routes.CheckCookie)
	http.HandleFunc(pathPrefix+"/api/v1/addHost", routes.AddHost)
	http.HandleFunc(pathPrefix+"/api/v1/addLogLine", routes.AddLogLine)
	http.HandleFunc(pathPrefix+"/api/v1/changeFavourite", routes.ChangeFavourite)
	http.HandleFunc(pathPrefix+"/api/v1/createUser", routes.CreateUser)
	http.HandleFunc(pathPrefix+"/api/v1/getSecret", routes.GetSecret)
	http.HandleFunc(pathPrefix+"/api/v1/getHosts", routes.GetHosts)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByService", routes.GetSizeByService)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByAll", routes.GetSizeByAll)
	http.HandleFunc(pathPrefix+"/api/v1/getLogs", routes.GetLogs)
	http.HandleFunc(pathPrefix+"/api/v1/getUsers", routes.GetUsers)
	http.HandleFunc(pathPrefix+"/api/v1/getLogsStream", routes.GetLogsStream)
	http.HandleFunc(pathPrefix+"/api/v1/login", routes.Login)
	http.HandleFunc(pathPrefix+"/api/v1/logout", routes.Logout)
	http.HandleFunc(pathPrefix+"/api/v1/editUser", routes.EditUser)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainerLogs", routes.DeleteContainerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainer", routes.DeleteContainer)
	http.HandleFunc(pathPrefix+"/api/v1/deleteUser", routes.DeleteUser)

	fmt.Println("ONLOGS: ", http.ListenAndServe(":"+string(os.Getenv("PORT")), nil))
}
