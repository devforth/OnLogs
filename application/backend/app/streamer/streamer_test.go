package streamer

import (
	"context"
	"fmt"
	"testing"
)

func TestRegisterStatisticsWorkerNoDuplicates(t *testing.T) {
	ctrl := &StreamController{}
	location := getStatsWorkerKey("host", "container")

	first := ctrl.registerStatisticsWorker(location, func() {})
	second := ctrl.registerStatisticsWorker(location, func() {})

	if !first {
		t.Fatal("first registration must succeed")
	}
	if second {
		t.Fatal("duplicate registration must be rejected")
	}
	if ctrl.statisticsWorkersCount() != 1 {
		t.Fatalf("expected exactly one worker, got %d", ctrl.statisticsWorkersCount())
	}
}

func TestStatisticsWorkersLongChurnDoesNotLeak(t *testing.T) {
	ctrl := &StreamController{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host := "churn-host"
	for i := 0; i < 300; i++ {
		container := fmt.Sprintf("ephemeral-%d", i)
		ctrl.ensureStatisticsWorker(ctx, host, container)
		ctrl.stopStatisticsWorker(host, container)
	}

	if ctrl.statisticsWorkersCount() != 0 {
		t.Fatalf("expected zero workers after churn, got %d", ctrl.statisticsWorkersCount())
	}
}
