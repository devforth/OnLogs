package streamer

import (
	"context"
	"fmt"
	"testing"

	"github.com/devforth/OnLogs/app/statistics"
)

// The registry moved into the statistics package so the docker streamer and the
// agent ingestion route share one worker per host/container.
func TestRegisterStatisticsWorkerNoDuplicates(t *testing.T) {
	ctrl := &StreamController{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseline := ctrl.statisticsWorkersCount()
	t.Cleanup(func() { ctrl.stopStatisticsWorker("host", "container") })

	first := statistics.EnsureWorker(ctx, "host", "container")
	second := statistics.EnsureWorker(ctx, "host", "container")

	if !first {
		t.Fatal("first registration must succeed")
	}
	if second {
		t.Fatal("duplicate registration must be rejected")
	}
	if got := ctrl.statisticsWorkersCount(); got != baseline+1 {
		t.Fatalf("expected exactly one new worker, got %d (baseline %d)", got, baseline)
	}
}

func TestStatisticsWorkersLongChurnDoesNotLeak(t *testing.T) {
	ctrl := &StreamController{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	baseline := ctrl.statisticsWorkersCount()
	host := "churn-host"
	for i := 0; i < 300; i++ {
		container := fmt.Sprintf("ephemeral-%d", i)
		ctrl.ensureStatisticsWorker(ctx, host, container)
		ctrl.stopStatisticsWorker(host, container)
	}

	if got := ctrl.statisticsWorkersCount(); got != baseline {
		t.Fatalf("expected %d workers after churn, got %d", baseline, got)
	}
}
