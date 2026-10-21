package daemon

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func corpusLine(index int) string {
	ts := time.Date(2026, 2, 10, 12, 44, 3, index*1000000, time.UTC)
	return ts.Format(time.RFC3339Nano) + fmt.Sprintf(" line-%03d", index)
}

func storedRowCount(t *testing.T, host, container string) int {
	t.Helper()
	db := util.GetDB(host, container, "logs")
	if db == nil {
		t.Fatal("no logs database")
	}
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	count := 0
	for iter.Next() {
		count++
	}
	return count
}

// A container's stream is attached, lines are ingested, the process dies, and a
// new process attaches again. Docker's Since is floored to whole seconds and the
// resume cursor is deliberately rewound, so the second attach replays lines that
// are already stored. The in-memory dedupe ring does not survive the restart.
func TestRestartMidStreamStoresEachLineExactlyOnce(t *testing.T) {
	host := util.GetHost()
	container := "RestartUnderLoad"
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container) })

	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, host+"/"+container)
	vars.Mutex.Unlock()

	const totalLines = 60
	const restartAfter = 40
	const replayFrom = 20 // the overlap window the rewound cursor re-serves

	emitted := map[string]struct{}{}

	first := NewDaemonService(nil)
	db := util.GetDB(host, container, "logs")
	if db == nil {
		t.Fatal("could not open the logs database")
	}

	storedThrough := first.storedThrough(host, container)
	first.seedBoundaryFingerprints(host, container, storedThrough)
	for i := 0; i < restartAfter; i++ {
		line := corpusLine(i)
		emitted[line] = struct{}{}
		first.ingestLine(host, container, db, "", false, storedThrough, line)
	}

	if got := storedRowCount(t, host, container); got != restartAfter {
		t.Fatalf("first run stored %d rows, expected %d", got, restartAfter)
	}

	second := NewDaemonService(nil)

	restartCursor := second.storedThrough(host, container)
	if restartCursor.IsZero() {
		t.Fatal("no cursor was persisted, so a restart cannot know where it left off")
	}
	second.seedBoundaryFingerprints(host, container, restartCursor)

	for i := replayFrom; i < totalLines; i++ {
		line := corpusLine(i)
		emitted[line] = struct{}{}
		second.ingestLine(host, container, db, "", false, restartCursor, line)
	}

	stored := storedRowCount(t, host, container)
	if stored != len(emitted) {
		t.Fatalf("stored %d rows for %d distinct emitted lines: %d duplicate row(s) survived the restart",
			stored, len(emitted), stored-len(emitted))
	}
	if stored != totalLines {
		t.Fatalf("stored %d rows, expected %d", stored, totalLines)
	}

	if dropped := second.DroppedReplays(container); dropped != restartAfter-replayFrom {
		t.Errorf("expected %d replayed lines to be dropped and counted, got %d",
			restartAfter-replayFrom, dropped)
	}

	assertNoDuplicateMessages(t, host, container)
}

func assertNoDuplicateMessages(t *testing.T, host, container string) {
	t.Helper()
	db := util.GetDB(host, container, "logs")
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	seen := map[string]struct{}{}
	for iter.Next() {
		message := string(iter.Value())
		if _, exists := seen[message]; exists {
			t.Errorf("message stored more than once: %q", message)
		}
		seen[message] = struct{}{}
	}
}

func TestRestartKeepsADistinctLineSharingTheCursorTimestamp(t *testing.T) {
	host := util.GetHost()
	container := "SameNanosecondBoundary"
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container) })

	db := util.GetDB(host, container, "logs")
	if db == nil {
		t.Fatal("could not open the logs database")
	}

	ts := time.Date(2026, 2, 10, 12, 44, 3, 123456789, time.UTC).Format(time.RFC3339Nano)
	first := NewDaemonService(nil)

	if stored, _ := first.ingestLine(host, container, db, "", false, time.Time{}, ts+" first at this nanosecond"); !stored {
		t.Fatal("the first line was not stored")
	}

	second := NewDaemonService(nil)
	cursor := second.storedThrough(host, container)
	second.seedBoundaryFingerprints(host, container, cursor)

	if stored, _ := second.ingestLine(host, container, db, "", false, cursor, ts+" first at this nanosecond"); stored {
		t.Error("a replayed line at the cursor timestamp was stored again")
	}
	if stored, _ := second.ingestLine(host, container, db, "", false, cursor, ts+" second at this nanosecond"); !stored {
		t.Error("a distinct line sharing the cursor's nanosecond was silently discarded")
	}

	assertNoDuplicateMessages(t, host, container)
	if got := storedRowCount(t, host, container); got != 2 {
		t.Fatalf("expected both distinct lines to be stored, got %d rows", got)
	}
}

func TestRunningNamesSkipsExitedContainers(t *testing.T) {
	result := []docker.ContainerNamesResult{
		{Name: "alive", ID: "1", Running: true},
		{Name: "exited", ID: "2", Running: false},
		{Name: "", ID: "3", Running: true},
		{Name: "alive-too", ID: "4", Running: true},
	}

	names := runningNames(result)

	if len(names) != 2 || names[0] != "alive" || names[1] != "alive-too" {
		t.Fatalf("expected only the running, named containers, got %v", names)
	}
}

func TestRunningNamesSkipsUnsafeNames(t *testing.T) {
	result := []docker.ContainerNamesResult{
		{Name: "app/redis", ID: "1", Running: true},
		{Name: "..", ID: "2", Running: true},
		{Name: "redis", ID: "3", Running: true},
	}

	names := runningNames(result)

	if len(names) != 1 || names[0] != "redis" {
		t.Fatalf("expected only the safe name, got %v", names)
	}
}

func TestIngestRetriesALineThatFailedToPersist(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HOST", "http://127.0.0.1:1")
	host := util.GetHost()
	line := "2026-10-19T12:00:00.000000001Z unpersisted"
	lineTS, _ := time.Parse(time.RFC3339Nano, "2026-10-19T12:00:00.000000001Z")

	for _, agentMode := range []bool{false, true} {
		container := "IngestRetryLocal"
		if agentMode {
			container = "IngestRetryAgent"
		}
		t.Cleanup(func() {
			for _, kind := range []string{"logs", "statuses", "brokenlogs", "streamstate"} {
				util.ResetDB(host, container, kind)
			}
		})
		var logs *leveldb.DB
		if agentMode {
			if q := util.GetDB(host, container, "brokenlogs"); q == nil || q.Close() != nil {
				t.Fatal("could not close the queue")
			}
		}
		h := NewDaemonService(nil)

		if stored, err := h.ingestLine(host, container, logs, "", agentMode, time.Time{}, line); stored || err == nil {
			t.Errorf("agent=%t: a line that was not persisted was reported as handled: stored=%t err=%v", agentMode, stored, err)
		}
		if cursor := h.storedThrough(host, container); !cursor.Before(lineTS) {
			t.Errorf("agent=%t: cursor %v is not before the line that was not persisted", agentMode, cursor)
		}
		if since := h.getResumeSince(host, container); since.After(lineTS) {
			t.Errorf("agent=%t: the next attach starts at %v, after the line that was not persisted", agentMode, since)
		}

		util.ResetDB(host, container, "brokenlogs")
		logs = util.GetDB(host, container, "logs")
		if stored, err := h.ingestLine(host, container, logs, "", agentMode, h.storedThrough(host, container), line); !stored || err != nil {
			t.Errorf("agent=%t: the retry was dropped: stored=%t err=%v", agentMode, stored, err)
		}
	}
}
