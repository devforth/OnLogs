package streamer

import (
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/vars"
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

func StreamLogs() {
	containers := daemon.GetContainersList()
	vars.All_Containers = containers
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
