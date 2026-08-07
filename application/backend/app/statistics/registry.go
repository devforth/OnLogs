package statistics

import (
	"context"
	"sync"
)

// One statistics worker per host/container, shared by the docker streamer and
// the agent ingestion route. Without a single registry each ingested log line
// spawned another immortal worker, and every new worker zeroes the live counter.
type workerHandle struct {
	cancel context.CancelFunc
	id     uint64
}

var (
	workersMutex sync.Mutex
	workerSeq    uint64
	workers      = map[string]*workerHandle{}
)

func WorkerKey(host string, container string) string {
	return host + "/" + container
}

func registerWorker(location string, cancel context.CancelFunc) (uint64, bool) {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	if _, exists := workers[location]; exists {
		return 0, false
	}
	workerSeq++
	workers[location] = &workerHandle{cancel: cancel, id: workerSeq}
	return workerSeq, true
}

// A worker can take seconds to unwind after its context is cancelled, by which
// time a replacement may already be registered. Only retire our own.
func retireWorker(location string, id uint64) {
	workersMutex.Lock()
	defer workersMutex.Unlock()

	if handle, exists := workers[location]; exists && handle.id == id {
		delete(workers, location)
	}
}

// EnsureWorker starts a worker for host/container unless one already runs.
func EnsureWorker(ctx context.Context, host string, container string) bool {
	location := WorkerKey(host, container)
	workerCtx, cancel := context.WithCancel(ctx)

	id, registered := registerWorker(location, cancel)
	if !registered {
		cancel()
		return false
	}

	go func() {
		defer cancel()
		defer retireWorker(location, id)
		RunStatisticForContainerWithContext(workerCtx, host, container)
	}()
	return true
}

func StopWorker(host string, container string) {
	location := WorkerKey(host, container)

	workersMutex.Lock()
	handle, exists := workers[location]
	if exists {
		delete(workers, location)
	}
	workersMutex.Unlock()

	if exists {
		handle.cancel()
	}
}

func WorkerCount() int {
	workersMutex.Lock()
	defer workersMutex.Unlock()
	return len(workers)
}
