package statistics

import (
	"context"
	"sync"
)

// One statistics worker per host/container, shared by the docker streamer and
// the agent ingestion route. Without a single registry each ingested log line
// spawned another immortal worker, and every new worker zeroes the live counter.
var (
	workersMutex sync.Mutex
	workers      = map[string]context.CancelFunc{}
)

func WorkerKey(host string, container string) string {
	return host + "/" + container
}

func registerWorker(location string, cancel context.CancelFunc) bool {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	if _, exists := workers[location]; exists {
		return false
	}
	workers[location] = cancel
	return true
}

// EnsureWorker starts a worker for host/container unless one already runs.
func EnsureWorker(ctx context.Context, host string, container string) bool {
	location := WorkerKey(host, container)
	workerCtx, cancel := context.WithCancel(ctx)

	if !registerWorker(location, cancel) {
		cancel()
		return false
	}

	go func() {
		defer StopWorker(host, container)
		RunStatisticForContainerWithContext(workerCtx, host, container)
	}()
	return true
}

func StopWorker(host string, container string) {
	location := WorkerKey(host, container)

	workersMutex.Lock()
	cancel, exists := workers[location]
	if exists {
		delete(workers, location)
	}
	workersMutex.Unlock()

	if exists {
		cancel()
	}
}

func WorkerCount() int {
	workersMutex.Lock()
	defer workersMutex.Unlock()
	return len(workers)
}
