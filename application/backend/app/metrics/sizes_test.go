package metrics

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
)

// n container directories on disk that the scrape must ignore.
func containerDirsOnDisk(t testing.TB, host string, n int) {
	t.Helper()

	base := "leveldb/hosts/" + host + "/containers/"
	for i := 0; i < n; i++ {
		path := base + "bench" + strconv.Itoa(i)
		if err := os.MkdirAll(path+"/logs", 0700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path+"/logs/CURRENT", []byte("x"), 0600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	t.Cleanup(func() { os.RemoveAll("leveldb/hosts/" + host) })
}

func countSeries(body string, name string) int {
	count := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+"{") {
			count++
		}
	}
	return count
}

// 200 containers on disk while the snapshot holds 2: a handler that walked would
// emit 200 series. Deterministic, unlike timing it.
func TestScrapeIgnoresContainersOnDisk(t *testing.T) {
	containerDirsOnDisk(t, "ignored-host", 200)

	setSizes(&sizeSnapshot{
		perContainer: map[string]int64{
			"cached-host/cached-a": 1024,
			"cached-host/cached-b": 2048,
		},
		total: 3072,
	})
	t.Cleanup(func() { setSizes(nil) })

	out := body(t, nil)

	if got := countSeries(out, "onlogs_container_db_size_bytes"); got != 2 {
		t.Errorf("container size series: got %d, want 2 (the snapshot); a walk would give 200", got)
	}
	if value, ok := sample(t, out, "onlogs_logs_size_bytes"); !ok || value != 3072 {
		t.Errorf("onlogs_logs_size_bytes: got %v (present=%v), want 3072", value, ok)
	}
	if value, ok := sample(t, out, `onlogs_container_db_size_bytes{host="cached-host",container="cached-a"}`); !ok || value != 1024 {
		t.Errorf("per-container size: got %v (present=%v), want 1024", value, ok)
	}
}

func TestSizeSnapshotBeforeFirstRefresh(t *testing.T) {
	setSizes(nil)
	t.Cleanup(func() { setSizes(nil) })

	done := make(chan string, 1)
	go func() { done <- body(t, nil) }()

	select {
	case out := <-done:
		if strings.Contains(out, "onlogs_container_db_size_bytes{") {
			t.Error("per-container size emitted with no snapshot")
		}
		if _, ok := sample(t, out, "onlogs_logs_size_bytes"); ok {
			t.Error("total size emitted with no snapshot")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler blocked waiting for the first refresh")
	}
}

func TestLogsSizeLimitFromEnv(t *testing.T) {
	os.Setenv("MAX_LOGS_SIZE", "2GB")
	defer os.Unsetenv("MAX_LOGS_SIZE")

	value, ok := sample(t, body(t, nil), "onlogs_logs_size_limit_bytes")
	if !ok {
		t.Fatal("onlogs_logs_size_limit_bytes missing")
	}
	if value != 2*1000*1000*1000 && value != 2*1024*1024*1024 {
		t.Errorf("onlogs_logs_size_limit_bytes: got %v, want 2GB in bytes", value)
	}
}

func TestLogsSizeLimitOmittedWhenUnparseable(t *testing.T) {
	os.Setenv("MAX_LOGS_SIZE", "not-a-size")
	defer os.Unsetenv("MAX_LOGS_SIZE")

	if _, ok := sample(t, body(t, nil), "onlogs_logs_size_limit_bytes"); ok {
		t.Error("limit emitted for an unparseable MAX_LOGS_SIZE")
	}
}

func TestRetentionDeletedCounterExported(t *testing.T) {
	before := containerdb.RetentionDeletedLines()
	containerdb.CountRetentionDeleted(5)

	value, ok := sample(t, body(t, nil), "onlogs_retention_deleted_lines_total")
	if !ok {
		t.Fatal("onlogs_retention_deleted_lines_total missing")
	}
	if value != float64(before+5) {
		t.Errorf("onlogs_retention_deleted_lines_total: got %v, want %v", value, before+5)
	}
}

// Snapshot of 200 containers with 200 more on disk: measures rendering, not walking.
func BenchmarkMetricsHandler(b *testing.B) {
	for _, onDisk := range []int{0, 200} {
		b.Run(strconv.Itoa(onDisk)+"-dirs-on-disk", func(b *testing.B) {
			containerDirsOnDisk(b, "bench-host-"+strconv.Itoa(onDisk), onDisk)

			snapshot := &sizeSnapshot{perContainer: map[string]int64{}}
			for i := 0; i < 200; i++ {
				snapshot.perContainer["h/c"+strconv.Itoa(i)] = int64(i * 1024)
				snapshot.total += int64(i * 1024)
			}
			setSizes(snapshot)
			b.Cleanup(func() { setSizes(nil) })

			os.Setenv("METRICS_TOKEN", testToken)
			b.Cleanup(func() { os.Unsetenv("METRICS_TOKEN") })

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchScrape()
			}
		})
	}
}
