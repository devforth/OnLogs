package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/docker/docker/client"
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

	if os.Getenv("DOCKER_HOST") == "" {
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}

	if os.Getenv("MAX_LOGS_SIZE") == "" {
		os.Setenv("MAX_LOGS_SIZE", "10GB")
	}

	fmt.Println("INFO: OnLogs configs done!")
}

func main() {
	godotenv.Load(".env")
	init_config()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Println("ERROR: unable to initialize Docker client:", err)
		return
	}
	defer cli.Close()
	fmt.Println("INFO: Docker client initialized with API version negotiation")

	dockerService := &docker.DockerService{
		Client: cli,
	}

	daemonService := &daemon.DaemonService{
		DockerClient: dockerService,
	}

	streamController := &streamer.StreamController{
		DaemonService: daemonService,
	}

	bgContext := context.Background()

	if os.Getenv("AGENT") != "" {
		streamController.StreamLogs(bgContext)
	}

	go db.DeleteUnusedTokens()
	go streamController.StreamLogs(bgContext)
	// go util.RunSpaceMonitoring()
	util.ReplacePrefixVariableForFrontend()
	util.CreateInitUser()

	// Initialize the "Controller" with its dependencies
	routerCtrl := &routes.RouteController{
		DockerService: dockerService,
		DaemonService: daemonService,
	}

	pathPrefix := os.Getenv("ONLOGS_PATH_PREFIX")
	http.HandleFunc(pathPrefix+"/", routerCtrl.Frontend)
	http.HandleFunc(pathPrefix+"/api/v1/addHost", routerCtrl.AddHost)
	http.HandleFunc(pathPrefix+"/api/v1/addLogLine", routerCtrl.AddLogLine)
	http.HandleFunc(pathPrefix+"/api/v1/askForDelete", routerCtrl.AskForDelete)
	http.HandleFunc(pathPrefix+"/api/v1/changeFavorite", routerCtrl.ChangeFavourite)
	http.HandleFunc(pathPrefix+"/api/v1/checkCookie", routerCtrl.CheckCookie)
	http.HandleFunc(pathPrefix+"/api/v1/createUser", routerCtrl.CreateUser)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainer", routerCtrl.DeleteContainer)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainerLogs", routerCtrl.DeleteContainerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteDockerLogs", routerCtrl.DeleteDockerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteUser", routerCtrl.DeleteUser)
	http.HandleFunc(pathPrefix+"/api/v1/editHostname", routerCtrl.EditHostname)
	http.HandleFunc(pathPrefix+"/api/v1/editUser", routerCtrl.EditUser)
	http.HandleFunc(pathPrefix+"/api/v1/getChartData", routerCtrl.GetChartData)
	http.HandleFunc(pathPrefix+"/api/v1/getDockerSize", routerCtrl.GetDockerSize)
	http.HandleFunc(pathPrefix+"/api/v1/getHosts", routerCtrl.GetHosts)
	http.HandleFunc(pathPrefix+"/api/v1/getLogWithPrev", routerCtrl.GetLogWithPrev)
	http.HandleFunc(pathPrefix+"/api/v1/getLogs", routerCtrl.GetLogs)
	http.HandleFunc(pathPrefix+"/api/v1/getLogsStream", routerCtrl.GetLogsStream)
	http.HandleFunc(pathPrefix+"/api/v1/getPrevLogs", routerCtrl.GetPrevLogs)
	http.HandleFunc(pathPrefix+"/api/v1/getSecret", routerCtrl.GetSecret)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByAll", routerCtrl.GetSizeByAll)
	http.HandleFunc(pathPrefix+"/api/v1/getSizeByService", routerCtrl.GetSizeByService)
	http.HandleFunc(pathPrefix+"/api/v1/getStats", routerCtrl.GetStats)
	http.HandleFunc(pathPrefix+"/api/v1/getStorageData", routerCtrl.GetStorageData)
	http.HandleFunc(pathPrefix+"/api/v1/getUserSettings", routerCtrl.GetUserSettings)
	http.HandleFunc(pathPrefix+"/api/v1/getUsers", routerCtrl.GetUsers)
	http.HandleFunc(pathPrefix+"/api/v1/login", routerCtrl.Login)
	http.HandleFunc(pathPrefix+"/api/v1/logout", routerCtrl.Logout)
	http.HandleFunc(pathPrefix+"/api/v1/updateUserSettings", routerCtrl.UpdateUserSettings)

	fmt.Println("Listening on port:", string(os.Getenv("PORT"))+"...")
	fmt.Println("ONLOGS: ", http.ListenAndServe(":"+string(os.Getenv("PORT")), nil))
}
