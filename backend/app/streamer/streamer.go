package streamer

import (
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/vars"
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
				vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
				go daemon.CreateDaemonToDBStream(container, vars.DB)
			}
		}
		time.Sleep(1 * time.Second)
		containers = daemon.GetContainersList()
		vars.All_Containers = containers
	}
}
