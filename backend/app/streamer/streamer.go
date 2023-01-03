package streamer

import (
	"os"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

func messageHandler(connection websocket.Conn, gotPong *bool) {
	_, m, _ := connection.ReadMessage()
	if string(m) == "PONG" {
		*gotPong = true
	}
}

func checkConnections() { // TODO improve
	for {
		for container := range vars.Connections {
			newConnectionsList := []websocket.Conn{}
			for connectionIdx, connection := range vars.Connections[container] {
				gotPong := false

				connection.WriteMessage(1, []byte("PING"))
				go messageHandler(connection, &gotPong)
				time.Sleep(1 * time.Minute)

				if !gotPong {
					newConnectionsList = append(vars.Connections[container][:connectionIdx], vars.Connections[container][connectionIdx+1:]...)
				} else {
					newConnectionsList = vars.Connections[container]
				}
			}
			vars.Connections[container] = newConnectionsList
			newConnectionsList = []websocket.Conn{}
		}
		time.Sleep(1 * time.Minute)
	}
}

func StreamLogs() {
	containers := daemon.GetContainersList()
	vars.DockerContainers = containers
	for {
		for _, container := range containers {
			if !util.Contains(container, vars.Active_Daemon_Streams) {
				newDB, _ := leveldb.OpenFile("leveldb/logs/"+container, nil)
				vars.ActiveDBs[container] = newDB
				vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
				if os.Getenv("CLIENT") != "" {
					go daemon.CreateDaemonToHostStream(container)
				} else {
					// go checkConnections()
					go daemon.CreateDaemonToDBStream(container)
				}
			}
		}
		time.Sleep(20 * time.Second)
		containers = daemon.GetContainersList()
		vars.DockerContainers = containers
	}
}
