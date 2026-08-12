package statistics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
)

// Workers zero the live counter on their own schedule, so seeding it from a
// test has to take the same lock they do.
func seedCounter(location string, stats map[string]uint64) {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()
	vars.Container_Stat_Counter[location] = stats
}

// One worker per host/container, shared by the docker streamer and the agent
// ingestion route.
func TestEnsureWorkerRejectsDuplicates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseline := WorkerCount()
	t.Cleanup(func() { StopWorker("host", "container") })

	first := EnsureWorker(ctx, "host", "container")
	second := EnsureWorker(ctx, "host", "container")

	if !first {
		t.Fatal("first registration must succeed")
	}
	if second {
		t.Fatal("duplicate registration must be rejected")
	}
	if got := WorkerCount(); got != baseline+1 {
		t.Fatalf("expected exactly one new worker, got %d (baseline %d)", got, baseline)
	}
}

func TestWorkerChurnDoesNotLeak(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseline := WorkerCount()
	host := "churn-host"
	for i := 0; i < 300; i++ {
		container := fmt.Sprintf("ephemeral-%d", i)
		EnsureWorker(ctx, host, container)
		StopWorker(host, container)
	}

	if got := WorkerCount(); got != baseline {
		t.Fatalf("expected %d workers after churn, got %d", baseline, got)
	}
}

func TestWorkerCreatesItsCounterAndDatabase(t *testing.T) {
	const location = "Test/TestContainer"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunStatisticForContainerWithContext(ctx, "Test", "TestContainer")

	// Polled, not slept: under -race the worker takes far longer to reach its
	// first flush than any fixed delay worth hard-coding.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		vars.DBMutex.RLock()
		statsDB := vars.Stat_Containers_DBs[location]
		vars.DBMutex.RUnlock()

		vars.Mutex.Lock()
		_, counted := vars.Container_Stat_Counter[location]
		vars.Mutex.Unlock()

		if statsDB != nil && counted {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the worker never created its counter and statistics database")
}
