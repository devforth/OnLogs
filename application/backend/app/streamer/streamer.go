package streamer

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func createStreams(containers []string) {
	for _, container := range vars.DockerContainers {
		if !util.Contains(container, vars.Active_Daemon_Streams) {
			go statistics.RunStatisticForContainer(util.GetHost(), container)
			vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
			if os.Getenv("AGENT") != "" {
				go daemon.CreateDaemonToHostStream(container)
			} else {
				go daemon.CreateDaemonToDBStream(container)
			}
		}
	}
}

func StreamLogs() {
	if vars.FavsDBErr != nil || vars.StateDBErr != nil || vars.UsersDBErr != nil {
		fmt.Println("ERROR: unable to open leveldb", vars.FavsDBErr, vars.StateDBErr, vars.UsersDBErr)
		return
	}

	vars.DockerContainers = daemon.GetContainersList()
	if os.Getenv("AGENT") != "" {
		agent.SendInitRequest(vars.DockerContainers)
	}
	for {
		createStreams(vars.DockerContainers)
		time.Sleep(20 * time.Second)
		vars.Year = strconv.Itoa(time.Now().UTC().Year())
		vars.DockerContainers = daemon.GetContainersList()
		if os.Getenv("AGENT") != "" {
			agent.SendUpdate(vars.DockerContainers)
			agent.TryResend()
		}
	}
}
