package statistics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
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
	os.MkdirAll("leveldb/hosts/Test/containers/TestContainer", 0700)

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

func TestRecreatedContainerCountsEachLineOnce(t *testing.T) {
	host, svc := "RecreateHost", "svc"
	location := WorkerKey(host, svc)
	os.RemoveAll("leveldb/hosts/" + host)

	resets := make(chan struct{}, 8)
	SetBeforeStatsReset(func(l string) {
		if l == location {
			resets <- struct{}{}
		}
	})
	t.Cleanup(func() {
		SetBeforeStatsReset(nil)
		SetBeforeStatsSave(nil)
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, svc, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, location)
		vars.Mutex.Unlock()
	})

	logs := util.GetDB(host, svc, "logs")
	at, stored := time.Now().UTC().Add(-time.Minute), 0
	put := func(line string) {
		at = at.Add(time.Millisecond)
		if err := containerdb.PutLogMessage(logs, host, svc, []string{at.Format(fixedWidthNanos), line}); err != nil {
			t.Error(err)
		}
		stored++
	}

	baseline := WorkerCount()
	put("INFO old container line")
	if !EnsureWorker(context.Background(), host, svc) {
		t.Fatal("the old worker was not registered")
	}
	<-resets
	for i := 0; i < 3; i++ {
		put("INFO old container line")
	}

	replaced, registered := false, false
	SetBeforeStatsSave(func(l string) {
		if l != location || replaced {
			return
		}
		replaced = true
		put("INFO new container line")
		registered = EnsureWorker(context.Background(), host, svc)
		time.Sleep(100 * time.Millisecond)
	})
	StopWorker(host, svc)
	<-resets
	if !registered {
		t.Fatal("the replacement worker was not registered")
	}
	<-resets
	defer func() {
		StopWorker(host, svc)
		<-resets
	}()

	if got := WorkerCount(); got != baseline+1 {
		t.Errorf("%d workers after the recreate, want %d", got, baseline+1)
	}
	var chart, service int
	for _, levels := range GetChartData(host, svc, "month", 12) {
		for _, n := range levels {
			chart += int(n)
		}
	}
	for _, n := range GetStatisticsByService(host, svc, 1344) {
		service += int(n)
	}
	if chart != stored || service != stored {
		t.Errorf("stored %d lines, statistics report chart=%d service=%d", stored, chart, service)
	}
}
