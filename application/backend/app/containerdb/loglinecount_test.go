package containerdb

import (
	"os"
	"strconv"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func resetLogLineCounts() {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()
	logLineCounts = map[string]map[string]uint64{}
	logLineCapReported = false
}

func TestLogLineCountsIsDeepCopy(t *testing.T) {
	resetLogLineCounts()
	defer resetLogLineCounts()

	countLogStatus("copy-host/copy-container", "error")

	snapshot := LogLineCounts()
	snapshot["copy-host/copy-container"]["error"] = 99999

	if got := LogLineCounts()["copy-host/copy-container"]["error"]; got != 1 {
		t.Errorf("mutating the snapshot changed the source: got %d, want 1", got)
	}
}

// Every level from the first line, so rate() has no gap.
func TestLogLineCountsEmitsEveryLevel(t *testing.T) {
	resetLogLineCounts()
	defer resetLogLineCounts()

	countLogStatus("levels-host/levels-container", "error")

	counts := LogLineCounts()["levels-host/levels-container"]
	for _, level := range []string{"error", "warn", "debug", "info", "meta", "other"} {
		if _, ok := counts[level]; !ok {
			t.Errorf("level %q missing from the snapshot", level)
		}
	}
	if counts["info"] != 0 {
		t.Errorf("unseen level info: got %d, want 0", counts["info"])
	}
	if counts["error"] != 1 {
		t.Errorf("seen level error: got %d, want 1", counts["error"])
	}
}

func TestLogLineCountsCapsTrackedContainers(t *testing.T) {
	resetLogLineCounts()
	defer resetLogLineCounts()

	for i := 0; i < maxTrackedContainers+50; i++ {
		countLogStatus("host/c"+strconv.Itoa(i), "error")
	}

	if got := len(LogLineCounts()); got != maxTrackedContainers {
		t.Errorf("tracked containers: got %d, want %d", got, maxTrackedContainers)
	}

	// An already-tracked container keeps counting past the cap.
	countLogStatus("host/c0", "error")
	if got := LogLineCounts()["host/c0"]["error"]; got != 2 {
		t.Errorf("tracked container stopped counting at the cap: got %d, want 2", got)
	}
}

func TestDeleteContainerDropsCounter(t *testing.T) {
	const host, container = "drop-host", "drop-container"
	location := host + "/" + container
	defer os.RemoveAll("leveldb/hosts/" + host)

	resetLogLineCounts()
	defer resetLogLineCounts()

	countLogStatus(location, "error")
	if _, ok := LogLineCounts()[location]; !ok {
		t.Fatal("counter missing before delete")
	}

	DeleteContainer(host, container, true)

	if _, ok := LogLineCounts()[location]; ok {
		t.Error("DeleteContainer left the counter in place; it must delete the key")
	}
}
