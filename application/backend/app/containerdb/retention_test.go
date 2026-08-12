package containerdb

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestRetentionMeasuresOnlyWhatItCanPrune(t *testing.T) {
	host, container := "RetentionHost", "RetentionContainer"
	base := "leveldb/hosts/" + host + "/containers/" + container
	_ = os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host) })

	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, host+"/"+container)
	vars.Mutex.Unlock()

	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("could not open the logs database")
	}
	for i := 0; i < 5; i++ {
		ts := testYear + "-02-10T12:5" + string(rune('0'+i)) + ":09.230421754Z"
		if err := PutLogMessage(logsDB, host, container, []string{ts, "a log line"}); err != nil {
			t.Fatal(err)
		}
	}

	withLogsOnly := util.GetPrunableSize(host, container)
	if withLogsOnly == 0 {
		t.Fatal("prunable size is zero even though logs were written")
	}

	// Something retention cannot delete from: a large broken-logs buffer.
	if err := os.MkdirAll(base+"/brokenlogs", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+"/brokenlogs/ballast", make([]byte, 8*1024*1024), 0o600); err != nil {
		t.Fatal(err)
	}

	afterBallast := util.GetPrunableSize(host, container)
	if afterBallast != withLogsOnly {
		t.Errorf("a sub-database retention cannot prune changed the measured size: %v -> %v",
			withLogsOnly, afterBallast)
	}

	if util.GetDirSize(host, container) <= afterBallast {
		t.Error("GetDirSize should still report the full directory, for the size display")
	}
}

func TestPrunableDBsCoverEverythingRetentionMeasures(t *testing.T) {
	prunable := util.PrunableDBs()
	for _, expected := range []string{"logs", "statuses", "statistics"} {
		found := false
		for _, actual := range prunable {
			if actual == expected {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is measured by retention but never pruned", expected)
		}
	}
}
