package util

import (
	"strings"

	daemon "github.com/devforth/OnLogs/app/daemon"
	db "github.com/devforth/OnLogs/app/db"
)

func StoreLogs() {
	containers := daemon.GetContainersList()
	for _, container := range containers {
		logs := strings.Split(daemon.GetAllContainerLogs(container), "\n")
		logs = logs[:len(logs)-1]
		for _, logLine := range logs {
			logItem := &db.LogItem{
				Datetime: logLine[:30],
				Message:  logLine[31 : len(logLine)-1],
			}
			db.StoreItem(container, logItem)
		}
	}
}

func getLogs() {
}
