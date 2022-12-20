package streamer

import (
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
)

func contains(a string, list []string) bool {
	for _, b := range list {
		if strings.Compare(b, a) == 0 {
			return true
		}
	}
	return false
}

func messageHandler(connection websocket.Conn, gotPong *bool) {
	_, m, _ := connection.ReadMessage()
	if string(m) == "PONG" {
		*gotPong = true
	}
}

func checkConnections() {
	for {
		for container, _ := range vars.Connections {
			newConnectionsList := []websocket.Conn{}
			for connectionIdx, connection := range vars.Connections[container] {
				gotPong := false

				connection.WriteMessage(1, []byte("PING"))
				go messageHandler(connection, &gotPong)
				time.Sleep(5 * time.Second)

				if !gotPong {
					newConnectionsList = append(vars.Connections[container][:connectionIdx], vars.Connections[container][connectionIdx+1:]...)
				} else {
					newConnectionsList = vars.Connections[container]
				}
			}
			vars.Connections[container] = newConnectionsList
			newConnectionsList = []websocket.Conn{}

		}
		time.Sleep(5 * time.Minute)
	}
}

func StreamLogs() {
	containers := daemon.GetContainersList()
	vars.All_Containers = containers
	go checkConnections()
	for {
		for _, container := range containers {
			if !contains(container, vars.Active_Daemon_Streams) {
				newDB, _ := leveldb.OpenFile("leveldb/"+container, nil)
				vars.ActiveDBs[container] = newDB
				vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
				go daemon.CreateDaemonToDBStream(container)
			}
		}
		time.Sleep(1 * time.Second)
		containers = daemon.GetContainersList()
		vars.All_Containers = containers
	}
}
