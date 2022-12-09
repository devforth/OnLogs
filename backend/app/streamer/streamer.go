package streamer

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/srchx_db"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
	"github.com/nsqio/go-diskqueue"
)

func contains(a string, list []string) bool {
	for _, b := range list {
		if strings.Compare(b, a) == 0 {
			return true
		}
	}
	return false
}

func NewAppLogger() diskqueue.AppLogFunc {
	return func(lvl diskqueue.LogLevel, f string, args ...interface{}) {
		log.Println(fmt.Sprintf(lvl.String()+": "+f, args...))
	}
}

func StreamLogs() {
	os.RemoveAll("/logDump")
	os.Mkdir("logDump", 0755)
	containers := daemon.GetContainersList()
	for {
		for _, container := range containers {
			containers_with_active_dq := make([]string, 0, len(vars.Active_DQ))
			for k := range vars.Active_DQ {
				containers_with_active_dq = append(containers_with_active_dq, k)
			}

			if !contains(container, containers_with_active_dq) {
				vars.Connections[container] = []websocket.Conn{}

				tmpDir, _ := ioutil.TempDir("logDump", container)
				dq := diskqueue.New(container, tmpDir, 4096, 4, 1<<10, 2500, 2*time.Second, NewAppLogger())
				vars.Active_DQ[container] = dq

				vars.All_Containers = append(vars.All_Containers, container)

				go CreateLogfileToDBStream(container, dq)
			}

			if !contains(container, vars.Active_Daemon_Streams) {
				vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, container)
				go daemon.CreateDaemonToLogfileStream(container, vars.Active_DQ[container])
			}
		}
		time.Sleep(1 * time.Second)
		containers = daemon.GetContainersList()
	}
}

func CreateLogfileToDBStream(containerName string, dq diskqueue.Interface) {
	for {
		content := dq.ReadChan()
		if content == nil {
			time.Sleep(1 * time.Second)
			continue
		}
		logLine := string(<-content)
		logItem := &srchx_db.LogItem{
			Datetime: logLine[:30],
			Message:  logLine[31 : len(logLine)-1],
		}
		for _, c := range vars.Connections[containerName] {
			c.WriteMessage(1, []byte(logLine))
		}

		srchx_db.StoreItem(containerName, logItem)
		time.Sleep(3 * time.Millisecond)
		continue
	}
}
