package main

import (
	"strings"
)

func storeLogs() {
	containers := getContainersList()
	for _, container := range containers {
		logs := strings.Split(getContainerLogs(container), "\n")
		logs = logs[:len(logs)-1]
		for _, logLine := range logs {
			logItem := &LogItem{
				datetime: logLine[:30],
				message:  logLine[31:],
			}
			storeItem(container, logItem)
		}
	}
}

func getLogs() {
}
