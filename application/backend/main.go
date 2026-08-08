package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/metrics"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/streamer"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
)

func ensureJWTSecret() error {
	if os.Getenv("JWT_SECRET") != "" {
		return nil
	}

	if persisted, err := os.ReadFile("leveldb/JWT_secret"); err == nil && len(persisted) > 0 {
		os.Setenv("JWT_SECRET", string(persisted))
		return nil
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("unable to generate a JWT secret: %w", err)
	}
	secret := hex.EncodeToString(buf)

	if err := os.MkdirAll("leveldb", 0700); err != nil {
		return fmt.Errorf("unable to create leveldb directory for the JWT secret: %w", err)
	}
	if err := os.WriteFile("leveldb/JWT_secret", []byte(secret), 0600); err != nil {
		return fmt.Errorf("unable to persist the generated JWT secret: %w", err)
	}

	os.Setenv("JWT_SECRET", secret)
	return nil
}

// gorilla clears the connection deadlines after hijacking (server.go:251), so
// WriteTimeout does not reach the websocket at /api/v1/getLogsStream.
func newServer(port string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

func init_config() {
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "2874")
	}

	if err := ensureJWTSecret(); err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(1)
	}

	if os.Getenv("DOCKER_HOST") == "" {
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}

	if os.Getenv("MAX_LOGS_SIZE") == "" {
		os.Setenv("MAX_LOGS_SIZE", "10GB")
	}
	if _, err := util.ParseHumanReadableSize(os.Getenv("MAX_LOGS_SIZE")); err != nil {
		fmt.Printf("FATAL: MAX_LOGS_SIZE=%q is not a valid size (%v); log retention would never run.\n",
			os.Getenv("MAX_LOGS_SIZE"), err)
		os.Exit(1)
	}

	fmt.Println("INFO: OnLogs configs done!")
}

func main() {
	godotenv.Load(".env")
	init_config()

	if err := vars.CheckDatabases(); err != nil {
		fmt.Println("FATAL:", err)
		os.Exit(1)
	}

	if os.Getenv("JWT_SECRET") == "" {
		fmt.Println("FATAL: JWT_SECRET is empty; refusing to start. Unset it to have one generated, or set a non-empty value.")
		os.Exit(1)
	}

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

	if util.IsAgentMode() {
		streamController.StreamLogs(bgContext)
	}

	go db.DeleteUnusedTokens()
	go metrics.StartSizeRefresher(bgContext)
	go streamController.StreamLogs(bgContext)
	// go util.RunSpaceMonitoring()
	util.ReplacePrefixVariableForFrontend()
	if err := util.CreateInitUser(); err != nil {
		if os.Getenv("DISABLE_AUTH") == "true" {
			fmt.Println("WARNING:", err, "(DISABLE_AUTH=true, continuing without an administrator account)")
		} else {
			fmt.Println("FATAL:", err)
			os.Exit(1)
		}
	}

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
	http.HandleFunc(pathPrefix+"/api/v1/createGroup", routerCtrl.CreateGroup)
	http.HandleFunc(pathPrefix+"/api/v1/createUser", routerCtrl.CreateUser)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainer", routerCtrl.DeleteContainer)
	http.HandleFunc(pathPrefix+"/api/v1/deleteContainerLogs", routerCtrl.DeleteContainerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteDockerLogs", routerCtrl.DeleteDockerLogs)
	http.HandleFunc(pathPrefix+"/api/v1/deleteGroup", routerCtrl.DeleteGroup)
	http.HandleFunc(pathPrefix+"/api/v1/deleteUser", routerCtrl.DeleteUser)
	http.HandleFunc(pathPrefix+"/api/v1/editUser", routerCtrl.EditUser)
	http.HandleFunc(pathPrefix+"/api/v1/getChartData", routerCtrl.GetChartData)
	http.HandleFunc(pathPrefix+"/api/v1/getDockerSize", routerCtrl.GetDockerSize)
	http.HandleFunc(pathPrefix+"/api/v1/getGroups", routerCtrl.GetGroups)
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
	// Registered even when METRICS_TOKEN is unset: the pathPrefix+"/" catch-all
	// below answers anything unregistered with index.html and a 200.
	http.HandleFunc(pathPrefix+"/api/v1/metrics", metrics.Handler(daemonService))
	http.HandleFunc(pathPrefix+"/api/v1/updateGroup", routerCtrl.UpdateGroup)
	http.HandleFunc(pathPrefix+"/api/v1/updateUserSettings", routerCtrl.UpdateUserSettings)

	fmt.Println("Listening on port:", string(os.Getenv("PORT"))+"...")
	fmt.Println("ONLOGS: ", newServer(os.Getenv("PORT"), nil).ListenAndServe())
}
