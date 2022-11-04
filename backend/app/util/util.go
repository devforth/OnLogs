package util

import (
	"io/ioutil"
	"os"
	"strconv"
	"time"

	daemon "github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
)

func StreamLogs() {
	containers := daemon.GetContainersList()
	for _, container := range containers {
		go daemon.CreateDaemonToLogfileStream(container)
		go CreateLogfileToDBStream(container)
	}
	// TODO check for containers upd
	// for {
	// 	newContainers := daemon.GetContainersList()
	// 	if !reflect.DeepEqual(containers, newContainers) {
	// 		for _, container := range containers {
	// 			daemon.CreateLogsStream(container)
	// 		}
	// 		containers = newContainers
	// 	}
	// 	fmt.Println(containers, newContainers)
	// 	time.Sleep(1 * time.Minute)
	// }
}

func CreateLogfileToDBStream(containerName string) {
	var counter uint64 = 0
	for {
		content, err := ioutil.ReadFile(containerName + "_logs/" + strconv.FormatUint(counter, 10) + "_log")
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		logLine := string(content)
		logItem := &db.LogItem{
			Datetime: logLine[:30],
			Message:  logLine[31 : len(logLine)-1],
		}
		db.StoreItem(containerName, logItem)
		os.Remove(containerName + "_logs/" + strconv.Itoa(int(counter)) + "_log")
		counter += 1
		time.Sleep(3 * time.Millisecond)
		continue
	}
}
