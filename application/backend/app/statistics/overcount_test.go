package statistics

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

// RFC3339Nano trims trailing zeros, which breaks both the 30-character minimum
// PutLogMessage enforces and lexicographic ordering in leveldb.
const fixedWidthNanos = "2006-01-02T15:04:05.000000000Z"

func persistedMeta(t *testing.T, host string, container string) uint64 {
	t.Helper()

	db := util.GetDB(host, container, "statistics")
	if db == nil {
		t.Fatal("no statistics database")
	}
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	var total uint64
	for iter.Next() {
		stats := map[string]uint64{}
		if err := json.Unmarshal(iter.Value(), &stats); err != nil {
			t.Fatalf("unparseable statistics record %q: %v", iter.Key(), err)
		}
		t.Logf("    %s  meta:%d", iter.Key(), stats["meta"])
		total += stats["meta"]
	}
	return total
}

// Drives restartStats the way the once-a-minute worker does. Each entry in
// arrivals is how many log lines landed during one flush interval.
func runFlushes(t *testing.T, name string, arrivals []int) (uint64, uint64) {
	t.Helper()

	host, container := "OverCount"+name, "svc"
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, container, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, host+"/"+container)
		vars.Mutex.Unlock()
	})
	os.MkdirAll("leveldb/hosts/"+host+"/containers/"+container, 0700)

	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("no logs database")
	}

	// The first flush stamps the cursor at now, so every line has to sort after it.
	restartStats(host, container)

	at := time.Now().UTC().Add(time.Hour)
	var written uint64
	for _, count := range arrivals {
		for i := 0; i < count; i++ {
			at = at.Add(time.Second)
			line := []string{at.Format(fixedWidthNanos), "ONLOGS: Container listening stopped! (unexpected EOF)"}
			if err := containerdb.PutLogMessage(logsDB, host, container, line); err != nil {
				t.Fatalf("put: %v", err)
			}
			written++
		}
		restartStats(host, container)
	}

	t.Logf("%s: %d log lines ->", name, written)
	return written, persistedMeta(t, host, container)
}

func TestStatsDoNotOverCountAcrossFlushes(t *testing.T) {
	arrivals := []int{}
	for i := 0; i < 14; i++ {
		arrivals = append(arrivals, 1, 0)
	}
	arrivals = append(arrivals, 1, 1, 0, 1, 1, 0)

	lines, persisted := runFlushes(t, "Drop", arrivals)
	if lines != 18 {
		t.Fatalf("meant to write 18 lines, wrote %d", lines)
	}
	if persisted != lines {
		t.Errorf("%d log lines are persisted as meta=%d", lines, persisted)
	}
}

func TestStatsDoNotUnderCountAfterIdleFlush(t *testing.T) {
	lines, persisted := runFlushes(t, "Burst", []int{1, 0, 3, 0, 0})
	if persisted != lines {
		t.Errorf("%d log lines are persisted as meta=%d", lines, persisted)
	}
}

func TestStatsCountLinesOlderThanTheFirstFlush(t *testing.T) {
	host, container := "LateIngest", "svc"
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, container, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, host+"/"+container)
		vars.Mutex.Unlock()
	})
	os.MkdirAll("leveldb/hosts/"+host+"/containers/"+container, 0700)

	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("no logs database")
	}

	restartStats(host, container)

	at := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		at = at.Add(time.Second)
		line := []string{at.Format(fixedWidthNanos), "ONLOGS: Container listening started!"}
		if err := containerdb.PutLogMessage(logsDB, host, container, line); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	restartStats(host, container)

	if persisted := persistedMeta(t, host, container); persisted != 3 {
		t.Errorf("3 log lines predating the first flush are persisted as meta=%d", persisted)
	}
}

func TestChartBucketsSumEveryRecordInThem(t *testing.T) {
	host, container := "ChartBucket", "svc"
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		util.ResetDB(host, container, "statistics")
		os.RemoveAll("leveldb/hosts/" + host)
	})
	os.MkdirAll("leveldb/hosts/"+host+"/containers/"+container, 0700)

	db := util.GetDB(host, container, "statistics")
	if db == nil {
		t.Fatal("no statistics database")
	}
	now := time.Now().UTC()
	for i, errors := range []uint64{7, 5} {
		record, _ := json.Marshal(map[string]uint64{"error": errors})
		key := now.Add(time.Duration(-i) * time.Second).Format(time.RFC3339Nano)
		if err := db.Put([]byte(key), record, nil); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}

	chart := GetChartData(host, container, "hour", 2)
	delete(chart, "now")

	var total uint64
	for bucket, stats := range chart {
		t.Logf("    %s  error:%d", bucket, stats["error"])
		total += stats["error"]
	}
	if total != 12 {
		t.Errorf("the two records hold 12 errors, the chart reports %d", total)
	}
}
