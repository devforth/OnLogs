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

func init_config() {
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "2874")
	}

	if os.Getenv("JWT_SECRET") == "" {
		token, err := os.ReadFile("leveldb/JWT_secret")
		if err != nil {
			os.WriteFile("leveldb/JWT_secret", []byte(os.Getenv("JWT_SECRET")), 0700)
			token, _ = os.ReadFile("leveldb/JWT_secret")
		}
		os.Setenv("JWT_SECRET", string(token))
	}

	if os.Getenv("DOCKER_SOCKET_PATH") == "" {
		os.Setenv("DOCKER_SOCKET_PATH", "/var/run/docker.sock")
	}

	if os.Getenv("MAX_LOGS_SIZE") == "" {
		os.Setenv("MAX_LOGS_SIZE", "5GB")
	}

	fmt.Println("INFO: OnLogs configs done!")
}

func main() {
	godotenv.Load(".env")
	init_config()
	if os.Getenv("AGENT") != "" {
		streamer.StreamLogs()
	}

	go db.DeleteUnusedTokens()
	go streamer.StreamLogs()
	// go util.RunSpaceMonitoring()
	util.ReplacePrefixVariableForFrontend()
	util.CreateInitUser()

	pathPrefix := os.Getenv("ONLOGS_PATH_PREFIX")
	http.HandleFunc(pathPrefix+"/", routes.Frontend)
	http.HandleFunc(pathPrefix+"/api/v1/addHost", routes.AddHost)
	http.HandleFunc(pathPrefix+"/api/v1/addLogLine", routes.AddLogLine)
	http.HandleFunc(pathPrefix+"/api/v1/askForDelete", routes.AskForDelete)
	http.HandleFunc(pathPrefix+"/api/v1/changeFavorite", routes.ChangeFavourite)
	http.HandleFunc(pathPrefix+"/api/v1/checkCookie", routes.CheckCookie)
	http.HandleFunc(pathPrefix+"/api/v1/createUser", routes.CreateUser)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainer", routes.DeleteContainer)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainerLogs", routes.DeleteContainerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteDockerLogs", routes.DeleteDockerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteUser", routes.DeleteUser)
	http.HandleFunc(pathPrefix+"/api/v1/editHostname", routes.EditHostname)
	http.HandleFunc(pathPrefix+"/api/v1/editUser", routes.EditUser)
	http.HandleFunc(pathPrefix+"/api/v1/getChartData", routes.GetChartData)
	http.HandleFunc(pathPrefix+"/api/v1/getDockerSize", routes.GetDockerSize)
	http.HandleFunc(pathPrefix+"/api/v1/getHosts", routes.GetHosts)
	http.HandleFunc(pathPrefix+"/api/v1/getLogWithPrev", routes.GetLogWithPrev)
	http.HandleFunc(pathPrefix+"/api/v1/getLogs", routes.GetLogs)
	http.HandleFunc(pathPrefix+"/api/v1/getLogsStream", routes.GetLogsStream)
	http.HandleFunc(pathPrefix+"/api/v1/getPrevLogs", routes.GetPrevLogs)
	http.HandleFunc(pathPrefix+"/api/v1/getSecret", routes.GetSecret)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByAll", routes.GetSizeByAll)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByService", routes.GetSizeByService)
	http.HandleFunc(pathPrefix+"/api/v1/getStats", routes.GetStats)
	http.HandleFunc(pathPrefix+"/api/v1/getStorageData", routes.GetStorageData)
	http.HandleFunc(pathPrefix+"/api/v1/getUserSettings", routes.GetUserSettings)
	http.HandleFunc(pathPrefix+"/api/v1/getUsers", routes.GetUsers)
	http.HandleFunc(pathPrefix+"/api/v1/login", routes.Login)
	http.HandleFunc(pathPrefix+"/api/v1/logout", routes.Logout)
	http.HandleFunc(pathPrefix+"/api/v1/updateUserSettings", routes.UpdateUserSettings)

	fmt.Println("Listening on port:", string(os.Getenv("PORT"))+"...")
	fmt.Println("ONLOGS: ", http.ListenAndServe(":"+string(os.Getenv("PORT")), nil))
}
