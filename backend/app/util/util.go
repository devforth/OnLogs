package util

import (
	"strings"

	db "github.com/devforth/OnLogs/app/db"
	requests "github.com/devforth/OnLogs/app/requests"
)

func StoreLogs() {
	containers := requests.GetContainersList()
	for _, container := range containers {
		logs := strings.Split(requests.GetContainerLogs(container, 0, 0), "\n")
		logs = logs[:len(logs)-1]
		for _, logLine := range logs {
			logItem := &db.LogItem{
				Datetime: logLine[:30],
				Message:  logLine[31:],
			}
			db.StoreItem(container, logItem)
		}
	}
}

func getLogs() {
}
