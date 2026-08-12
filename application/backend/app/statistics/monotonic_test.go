package statistics

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

// Lives here, not in containerdb, because resetInMemoryStats is unexported.
// Without this the counter resets every minute and rate() is meaningless.
func TestLogLineCounterSurvivesStatsReset(t *testing.T) {
	const host, container = "monotonic-host", "monotonic-container"
	location := host + "/" + container
	defer os.RemoveAll("leveldb/hosts/" + host)

	db := util.GetDB(host, container, "logs")
	if db == nil {
		t.Fatal("no logs db")
	}
	put := func(n int) {
		for i := 0; i < n; i++ {
			if err := containerdb.PutLogMessage(db, host, container, []string{
				"2026-08-08T10:00:00.000000000Z", "ERROR something failed",
			}); err != nil {
				t.Fatalf("PutLogMessage: %v", err)
			}
		}
	}

	before := containerdb.LogLineCounts()[location]["error"]
	put(10)
	if got := containerdb.LogLineCounts()[location]["error"]; got != before+10 {
		t.Fatalf("after 10 lines: got %d, want %d", got, before+10)
	}

	resetInMemoryStats(location)

	vars.Mutex.Lock()
	live := vars.Container_Stat_Counter[location]["error"]
	vars.Mutex.Unlock()
	if live != 0 {
		t.Fatalf("resetInMemoryStats left the live counter at %d, want 0", live)
	}

	put(10)
	if got := containerdb.LogLineCounts()[location]["error"]; got != before+20 {
		t.Errorf("across a stats reset: got %d, want %d", got, before+20)
	}
}
