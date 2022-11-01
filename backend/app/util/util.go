package util

import (
	daemon "github.com/devforth/OnLogs/app/daemon"
)

func StoreLogs() {
	containers := daemon.GetContainersList()
	for _, container := range containers {
		go daemon.CreateLogsStream(container)
	}
}
