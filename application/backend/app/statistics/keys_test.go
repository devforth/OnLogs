package statistics

import (
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestSavedStatisticsKeysParseAsTimestamps(t *testing.T) {
	host, container := "StatKeyHost", "StatKeyContainer"
	_ = os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host) })

	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, host+"/"+container)
	vars.Mutex.Unlock()

	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("could not open the logs database")
	}
	writeLines := func(base time.Time, count int) {
		for i := 0; i < count; i++ {
			ts := base.Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339Nano)
			if err := containerdb.PutLogMessage(logsDB, host, container, []string{ts, "ERROR something"}); err != nil {
				t.Fatal(err)
			}
		}
	}

	// First round takes the "no statistics yet" branch and saves under "now".
	writeLines(time.Now().Add(-time.Hour), 3)
	restartStats(host, container)

	// Lines arriving after that point drive the incremental branch, where the
	// cursor the forward scan returns becomes the key it saves under.
	writeLines(time.Now().Add(time.Minute), 3)
	restartStats(host, container)

	statsDB := util.GetDB(host, container, "statistics")
	if statsDB == nil {
		t.Fatal("no statistics database")
	}
	iter := statsDB.NewIterator(nil, nil)
	defer iter.Release()

	written := 0
	for iter.Next() {
		key := string(iter.Key())
		written++
		if _, err := time.Parse(time.RFC3339Nano, key); err != nil {
			t.Fatalf("statistics key %q is not a timestamp, so every chart point fails to parse: %v", key, err)
		}
	}
	if written == 0 {
		t.Fatal("no statistics were written at all")
	}
}

// The scan cursor is deliberately a full log key, so that paging can identify one
// row among several in the same instant.
func TestSaveStatsAlwaysWritesAParseableKey(t *testing.T) {
	host, container := "StatCursorHost", "StatCursorContainer"
	_ = os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host) })

	db := util.GetDB(host, container, "statistics")
	if db == nil {
		t.Fatal("could not open the statistics database")
	}

	fullLogKey := "2026-02-10T12:44:02.123456789Z +1786097928311912188-9"
	saveStats(db, emptyStats(), fullLogKey)
	saveStats(db, emptyStats(), "not a timestamp at all")

	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	written := 0
	for iter.Next() {
		written++
		key := string(iter.Key())
		if _, err := time.Parse(time.RFC3339Nano, key); err != nil {
			t.Errorf("saveStats wrote %q, which every reader fails to parse: %v", key, err)
		}
	}
	if written != 2 {
		t.Fatalf("expected 2 statistics entries, got %d", written)
	}
}
