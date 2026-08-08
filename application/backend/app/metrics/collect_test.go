package metrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

// app/daemon owns the tests for the real state; this covers rendering only.
type fakeDaemon struct {
	dropped map[string]uint64
	cursors map[string]time.Time
}

func (f fakeDaemon) DroppedReplayTotals() map[string]uint64 { return f.dropped }
func (f fakeDaemon) CursorTimestamps() map[string]time.Time { return f.cursors }

// Value of one series from a scrape, and whether it appeared.
func sample(t *testing.T, body string, series string) (float64, bool) {
	t.Helper()

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		i := strings.LastIndex(line, " ")
		if i < 0 || line[:i] != series {
			continue
		}
		value, err := strconv.ParseFloat(line[i+1:], 64)
		if err != nil {
			t.Fatalf("series %s has a non-numeric value %q", series, line[i+1:])
		}
		return value, true
	}
	return 0, false
}

func body(t *testing.T, ds daemonState) string {
	t.Helper()
	os.Setenv("METRICS_TOKEN", testToken)
	t.Cleanup(func() { os.Unsetenv("METRICS_TOKEN") })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rr := httptest.NewRecorder()
	Handler(ds).ServeHTTP(rr, req)
	return rr.Body.String()
}

func TestStreamUpTracksActiveStreams(t *testing.T) {
	host := util.GetHost()
	for _, c := range vars.ActiveStreams() {
		vars.RemoveActiveStream(c)
	}

	vars.AddActiveStream("metrics-stream-a")
	vars.AddActiveStream("metrics-stream-b")
	defer func() {
		vars.RemoveActiveStream("metrics-stream-a")
		vars.RemoveActiveStream("metrics-stream-b")
	}()

	out := body(t, nil)
	for _, container := range []string{"metrics-stream-a", "metrics-stream-b"} {
		series := `onlogs_stream_up{host="` + host + `",container="` + container + `"}`
		value, ok := sample(t, out, series)
		if !ok {
			t.Errorf("%s missing", series)
			continue
		}
		if value != 1 {
			t.Errorf("%s: got %v, want 1", series, value)
		}
	}

	vars.RemoveActiveStream("metrics-stream-a")
	if _, ok := sample(t, body(t, nil), `onlogs_stream_up{host="`+host+`",container="metrics-stream-a"}`); ok {
		t.Error("stopped stream still exported")
	}
}

func TestCursorTimestampOmittedWhenUnknown(t *testing.T) {
	if strings.Contains(body(t, fakeDaemon{}), "onlogs_stream_cursor_timestamp_seconds{") {
		t.Error("cursor series emitted with no cursors recorded; it must be omitted, not zero")
	}

	when := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	ds := fakeDaemon{cursors: map[string]time.Time{"cursor-host/cursor-container": when}}

	series := `onlogs_stream_cursor_timestamp_seconds{host="cursor-host",container="cursor-container"}`
	value, ok := sample(t, body(t, ds), series)
	if !ok {
		t.Fatalf("%s missing", series)
	}
	if value != float64(when.Unix()) {
		t.Errorf("%s: got %v, want %v", series, value, when.Unix())
	}
}

func TestStatsWorkersTracksRegistry(t *testing.T) {
	before, _ := sample(t, body(t, nil), "onlogs_stats_workers")

	statistics.EnsureWorker(context.Background(), "metrics-worker-host", "metrics-worker-container")
	defer statistics.StopWorker("metrics-worker-host", "metrics-worker-container")

	after, ok := sample(t, body(t, nil), "onlogs_stats_workers")
	if !ok {
		t.Fatal("onlogs_stats_workers missing")
	}
	if after != before+1 {
		t.Errorf("onlogs_stats_workers: got %v, want %v", after, before+1)
	}
}

func TestLoginCountersExported(t *testing.T) {
	vars.LoginFailures.Store(4)
	vars.LoginBlocked.Store(2)
	defer func() {
		vars.LoginFailures.Store(0)
		vars.LoginBlocked.Store(0)
	}()

	out := body(t, nil)
	if value, ok := sample(t, out, "onlogs_login_failures_total"); !ok || value != 4 {
		t.Errorf("onlogs_login_failures_total: got %v (present=%v), want 4", value, ok)
	}
	if value, ok := sample(t, out, "onlogs_login_blocked_total"); !ok || value != 2 {
		t.Errorf("onlogs_login_blocked_total: got %v (present=%v), want 2", value, ok)
	}
}

// Plausibility only: the exact values are not ours to predict.
func TestProcessGaugesPresent(t *testing.T) {
	out := body(t, nil)

	for _, series := range []string{
		"onlogs_goroutines",
		"onlogs_heap_bytes",
		"onlogs_filesystem_size_bytes",
		"onlogs_filesystem_avail_bytes",
		"onlogs_websocket_connections",
		"onlogs_metrics_scrape_duration_seconds",
	} {
		value, ok := sample(t, out, series)
		if !ok {
			t.Errorf("%s missing", series)
			continue
		}
		if value < 0 {
			t.Errorf("%s: got %v, want >= 0", series, value)
		}
	}

	if value, _ := sample(t, out, "onlogs_goroutines"); value < 1 {
		t.Errorf("onlogs_goroutines: got %v, want >= 1", value)
	}
	if value, _ := sample(t, out, "onlogs_heap_bytes"); value <= 0 {
		t.Errorf("onlogs_heap_bytes: got %v, want > 0", value)
	}
	if value, _ := sample(t, out, "onlogs_filesystem_size_bytes"); value <= 0 {
		t.Errorf("onlogs_filesystem_size_bytes: got %v, want > 0", value)
	}
}

func TestScrapeDurationIsPreviousScrape(t *testing.T) {
	scrapeDuration.Store(0)

	if value, _ := sample(t, body(t, nil), "onlogs_metrics_scrape_duration_seconds"); value != 0 {
		t.Errorf("first scrape: got %v, want 0", value)
	}
	if value, _ := sample(t, body(t, nil), "onlogs_metrics_scrape_duration_seconds"); value <= 0 {
		t.Errorf("second scrape: got %v, want > 0", value)
	}
}

func TestDroppedReplayExported(t *testing.T) {
	ds := fakeDaemon{dropped: map[string]uint64{"replay-host/replay-container": 1}}

	series := `onlogs_dropped_replay_lines_total{host="replay-host",container="replay-container"}`
	value, ok := sample(t, body(t, ds), series)
	if !ok {
		t.Fatalf("%s missing", series)
	}
	if value != 1 {
		t.Errorf("%s: got %v, want 1", series, value)
	}
}
