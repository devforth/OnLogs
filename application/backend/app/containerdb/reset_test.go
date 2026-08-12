package containerdb

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func TestDeleteContainerLeavesUsableHandlesBehind(t *testing.T) {
	host, container := "ResetHost", "ResetContainer"
	location := host + "/" + container
	base := "leveldb/hosts/" + host + "/containers/" + container
	_ = os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host) })

	// Warm the caches the way ingestion does.
	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("could not open the logs database")
	}
	util.GetDB(host, container, "statuses")
	util.GetDB(host, container, "statistics")

	if err := PutLogMessage(logsDB, host, container, []string{testYear + "-02-10T12:56:09.230421754Z", "before delete"}); err != nil {
		t.Fatal(err)
	}

	DeleteContainer(host, container, false)

	if _, err := os.Stat(base + "/active"); err == nil {
		t.Error("DeleteContainer created a stray active/ directory; later log writes go there and are never read back")
	}

	vars.DBMutex.RLock()
	statuses := vars.Statuses_DBs[location]
	stats := vars.Stat_Containers_DBs[location]
	vars.DBMutex.RUnlock()
	if statuses != nil && statuses == stats {
		t.Error("the statistics database was stored in the statuses slot")
	}

	reopened := util.GetDB(host, container, "logs")
	if reopened == nil {
		t.Fatal("the logs database could not be reopened after a delete")
	}
	if err := PutLogMessage(reopened, host, container, []string{testYear + "-02-10T12:57:09.230421754Z", "after delete"}); err != nil {
		t.Fatalf("ingestion failed after a delete: %v", err)
	}

	logs := GetLogs(false, false, host, container, "", 30, "", false, nil)["logs"].([][]string)
	if len(logs) != 1 || logs[0][1] != "after delete" {
		t.Fatalf("expected the post-delete line to be readable back, got %v", logs)
	}
}

func TestLogsOfSameNamedContainersOnDifferentHostsStaySeparate(t *testing.T) {
	container := "sharedname"
	for _, host := range []string{"HostAlpha", "HostBeta"} {
		_ = os.RemoveAll("leveldb/hosts/" + host)
	}
	t.Cleanup(func() {
		for _, host := range []string{"HostAlpha", "HostBeta"} {
			_ = os.RemoveAll("leveldb/hosts/" + host)
		}
	})

	for _, host := range []string{"HostAlpha", "HostBeta"} {
		db := util.GetDB(host, container, "logs")
		if db == nil {
			t.Fatalf("could not open the logs database for %s", host)
		}
		if err := PutLogMessage(db, host, container, []string{testYear + "-02-10T12:56:09.230421754Z", "line from " + host}); err != nil {
			t.Fatal(err)
		}
	}

	for _, host := range []string{"HostAlpha", "HostBeta"} {
		logs := GetLogs(false, false, host, container, "", 30, "", false, nil)["logs"].([][]string)
		if len(logs) != 1 {
			t.Fatalf("%s: expected exactly its own line, got %d rows: %v", host, len(logs), logs)
		}
		if logs[0][1] != "line from "+host {
			t.Fatalf("%s: read back another host's line: %q", host, logs[0][1])
		}
	}
}
