package streamer

import (
	"fmt"
	"os"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func createStreams(containers []string) {
	for _, container := range vars.DockerContainers {
		if !util.Contains(container, vars.Active_Daemon_Streams) {
			newDB, err := leveldb.OpenFile("leveldb/logs/"+container, nil)
			if err != nil {
				fmt.Println("ERROR: " + container + ": " + err.Error())
				newDB, err = leveldb.RecoverFile("leveldb/logs/"+container, nil)
				fmt.Println("INFO: " + container + ": recovering db...")
				if err == nil {
					fmt.Println("INFO: " + container + ": db recovered!")
				} else {
					fmt.Println("ERROR: " + container + ": " + err.Error())
				}
			}
			vars.ActiveDBs[container] = newDB
			vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
			if os.Getenv("CLIENT") != "" {
				go daemon.CreateDaemonToHostStream(container)
			} else {
				go daemon.CreateDaemonToDBStream(container)
			}
		}
	}
}

func StreamLogs() {
	vars.DockerContainers = daemon.GetContainersList()
	for {
		createStreams(vars.DockerContainers)
		time.Sleep(20 * time.Second)
		vars.DockerContainers = daemon.GetContainersList()
	}
}
