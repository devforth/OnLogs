package streamer

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	daemon "github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/srchx_db"
	"github.com/nsqio/go-diskqueue"
)

func NewAppLogger() diskqueue.AppLogFunc {
	return func(lvl diskqueue.LogLevel, f string, args ...interface{}) {
		log.Println(fmt.Sprintf(lvl.String()+": "+f, args...))
	}
}

func StreamLogs() {
	os.RemoveAll("/logDump")
	containers := daemon.GetContainersList()
	for _, container := range containers {
		tmpDir, _ := ioutil.TempDir("logDump", container)
		dq := diskqueue.New(container, tmpDir, 4096, 4, 1<<10, 2500, 2*time.Second, NewAppLogger())
		// defer dq.Close()

		go daemon.CreateDaemonToLogfileStream(container, dq)
		go CreateLogfileToDBStream(container, dq)
	}
}

func CreateLogfileToDBStream(containerName string, dq diskqueue.Interface) {
	for {
		content := dq.ReadChan()
		dq.Empty()
		if content == nil {
			time.Sleep(1 * time.Second)
			continue
		}
		logLine := string(<-content)
		logItem := &srchx_db.LogItem{
			Datetime: logLine[:30],
			Message:  logLine[31 : len(logLine)-1],
		}
		srchx_db.StoreItem(containerName, logItem)
		time.Sleep(3 * time.Millisecond)
		continue
	}
}
