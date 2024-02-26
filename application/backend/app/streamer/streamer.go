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
	"github.com/syndtr/goleveldb/leveldb"
)

func createStreams(containers []string) {
	for _, container := range vars.DockerContainers {
		if !util.Contains(container, vars.Active_Daemon_Streams) {
			go statistics.RunStatisticForContainer(util.GetHost(), container)
			newDB, err := leveldb.OpenFile("leveldb/hosts/"+util.GetHost()+"/containers/"+container+"/logs", nil)
			if err != nil {
				fmt.Println("ERROR: " + container + ": " + err.Error())
				newDB, err = leveldb.RecoverFile("leveldb/hosts/"+util.GetHost()+"/containers/"+container+"/logs", nil)
				fmt.Println("INFO: " + container + ": recovering db...")
				if err == nil {
					fmt.Println("INFO: " + container + ": db recovered!")
				} else {
					fmt.Println("ERROR: " + container + ": " + err.Error())
				}
			}
			if vars.Statuses_DBs[util.GetHost()+"/"+container] == nil {
				vars.Statuses_DBs[util.GetHost()+"/"+container] = util.GetDB(util.GetHost(), container, "/statuses")
			}
			vars.ActiveDBs[container] = newDB
			vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
			if os.Getenv("AGENT") != "" {
				vars.BrokenLogs_DBs[container] = util.GetDB(util.GetHost(), container, "/brokenlogs")
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
