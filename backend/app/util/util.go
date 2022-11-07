package util

import (
	"strconv"
	"time"

	daemon "github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/syndtr/goleveldb/leveldb"
)

func StreamLogs() {
	// for {
	containers := daemon.GetContainersList()
	for _, container := range containers {
		logDump, _ := leveldb.OpenFile("logDump/"+container, nil)
		go daemon.CreateDaemonToLogfileStream(container, logDump)
		go CreateLogfileToDBStream(container, logDump)
	}

	// new := daemon.GetContainersList()
	// for reflect.DeepEqual(containers, new) {
	// 	time.Sleep(15 * time.Second)
	// 	new = daemon.GetContainersList()
	// }

	// }
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

func CreateLogfileToDBStream(containerName string, logDump *leveldb.DB) {
	var counter uint64 = 0
	for {
		content, err := logDump.Get([]byte(strconv.FormatUint(counter, 10)), nil)
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
		logDump.Delete([]byte(strconv.FormatUint(counter, 10)), nil)
		logDump.Get([]byte(strconv.FormatUint(counter, 10)), nil)
		counter += 1
		// time.Sleep(3 * time.Millisecond)
		continue
	}
}
