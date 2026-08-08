package metrics

import (
	"fmt"
	"io"
	"math"
	"runtime"
	runtimemetrics "runtime/metrics"
	"sync/atomic"
	"time"

	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"golang.org/x/sys/unix"
)

// Not /gc/heap/live:bytes, which reads 0 until the first GC completes.
const heapMetric = "/memory/classes/heap/objects:bytes"

// The previous scrape's duration, as float64 bits: the current one cannot time
// itself, since the value goes into the body it is timing.
var scrapeDuration atomic.Uint64

func recordScrapeDuration(elapsed time.Duration) {
	scrapeDuration.Store(math.Float64bits(elapsed.Seconds()))
}

func lastScrapeDuration() float64 {
	return math.Float64frombits(scrapeDuration.Load())
}

// Local streams only: vars.Active_Daemon_Streams holds bare container names and
// never contains agent-forwarded ones.
func writeStreamUp(w io.Writer) {
	active := vars.ActiveStreams()
	if len(active) == 0 {
		return
	}

	host := util.GetHost()
	writeHeader(w, "onlogs_stream_up", "gauge", "1 while a local container's log stream is attached.")
	for _, container := range sortedStrings(active) {
		fmt.Fprintf(w, "onlogs_stream_up{host=\"%s\",container=\"%s\"} 1\n",
			escapeLabelValue(host), escapeLabelValue(container))
	}
}

// Raw timestamp, not a derived lag: a quiet container's lag grows forever and
// means nothing. The alert pairs this with a log-line rate.
func writeCursors(w io.Writer, ds daemonState) {
	if ds == nil {
		return
	}
	cursors := ds.CursorTimestamps()
	if len(cursors) == 0 {
		return
	}

	writeHeader(w, "onlogs_stream_cursor_timestamp_seconds", "gauge",
		"Timestamp of the newest log line ingested for a container.")
	for _, location := range sortedKeys(cursors) {
		host, container := splitLocation(location)
		fmt.Fprintf(w, "onlogs_stream_cursor_timestamp_seconds{host=\"%s\",container=\"%s\"} %d\n",
			escapeLabelValue(host), escapeLabelValue(container), cursors[location].Unix())
	}
}

func writeDroppedReplays(w io.Writer, ds daemonState) {
	if ds == nil {
		return
	}
	totals := ds.DroppedReplayTotals()
	if len(totals) == 0 {
		return
	}

	writeHeader(w, "onlogs_dropped_replay_lines_total", "counter",
		"Log lines dropped because a stream replayed something already stored.")
	for _, location := range sortedKeys(totals) {
		host, container := splitLocation(location)
		fmt.Fprintf(w, "onlogs_dropped_replay_lines_total{host=\"%s\",container=\"%s\"} %d\n",
			escapeLabelValue(host), escapeLabelValue(container), totals[location])
	}
}

func writeProcess(w io.Writer) {
	writeHeader(w, "onlogs_stats_workers", "gauge", "Statistics workers running, one per streamed container.")
	fmt.Fprintf(w, "onlogs_stats_workers %d\n", statistics.WorkerCount())

	writeHeader(w, "onlogs_websocket_connections", "gauge", "Live log viewers connected.")
	fmt.Fprintf(w, "onlogs_websocket_connections %d\n", vars.TotalConnections())

	writeHeader(w, "onlogs_goroutines", "gauge", "Goroutines running.")
	fmt.Fprintf(w, "onlogs_goroutines %d\n", runtime.NumGoroutine())

	samples := []runtimemetrics.Sample{{Name: heapMetric}}
	runtimemetrics.Read(samples)
	writeHeader(w, "onlogs_heap_bytes", "gauge", "Live heap objects in bytes.")
	fmt.Fprintf(w, "onlogs_heap_bytes %d\n", samples[0].Value.Uint64())

	writeHeader(w, "onlogs_login_failures_total", "counter", "Rejected login attempts.")
	fmt.Fprintf(w, "onlogs_login_failures_total %d\n", vars.LoginFailures.Load())

	writeHeader(w, "onlogs_login_blocked_total", "counter", "Login attempts refused by the rate limiter.")
	fmt.Fprintf(w, "onlogs_login_blocked_total %d\n", vars.LoginBlocked.Load())

	writeHeader(w, "onlogs_metrics_scrape_duration_seconds", "gauge",
		"Time the previous scrape took to render.")
	fmt.Fprintf(w, "onlogs_metrics_scrape_duration_seconds %g\n", lastScrapeDuration())
}

// Bavail, not Bfree: the difference is the root reserve, which OnLogs cannot use.
func writeFilesystem(w io.Writer) {
	var stat unix.Statfs_t
	if err := unix.Statfs(".", &stat); err != nil {
		return
	}

	writeHeader(w, "onlogs_filesystem_size_bytes", "gauge", "Size of the filesystem holding the logs.")
	fmt.Fprintf(w, "onlogs_filesystem_size_bytes %d\n", stat.Blocks*uint64(stat.Bsize))

	writeHeader(w, "onlogs_filesystem_avail_bytes", "gauge", "Space available to OnLogs on that filesystem.")
	fmt.Fprintf(w, "onlogs_filesystem_avail_bytes %d\n", stat.Bavail*uint64(stat.Bsize))
}
