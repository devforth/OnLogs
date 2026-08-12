package containerdb

import (
	"os"
	"strconv"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func seedLogs(t *testing.T, host, container string, n int) {
	t.Helper()
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	vars.Container_Stat_Counter[host+"/"+container] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}

	db, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	statusDB, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statuses", nil)
	if err != nil {
		t.Fatal(err)
	}
	vars.Statuses_DBs[host+"/"+container] = statusDB

	for i := 0; i < n; i++ {
		ts := testYear + "-02-10T12:" + pad(i/60) + ":" + pad(i%60) + ".230421754Z"
		if err := PutLogMessage(db, host, container, []string{ts, "line " + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	statusDB.Close()
	delete(vars.Statuses_DBs, host+"/"+container)
}

func pad(v int) string {
	s := strconv.Itoa(v)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

func TestGetLogsAlwaysReportsIsEnd(t *testing.T) {
	seedLogs(t, "LimitHost", "LimitCont", 5)

	for _, limit := range []int{-1, 0, 1, 5, 30} {
		result := GetLogs(false, false, "LimitHost", "LimitCont", "", limit, "", false, nil)
		if _, ok := result["is_end"]; !ok {
			t.Errorf("limit=%d: response omits is_end, so the client cannot tell it has finished", limit)
		}
	}
}

func TestGetLogsTreatsANonPositiveLimitAsADefaultPage(t *testing.T) {
	seedLogs(t, "LimitHost", "LimitCont", 5)

	for _, limit := range []int{0, -1, -1000} {
		logs := GetLogs(false, false, "LimitHost", "LimitCont", "", limit, "", false, nil)["logs"].([][]string)
		if len(logs) != 5 {
			t.Errorf("limit=%d returned %d rows, want all 5", limit, len(logs))
		}
	}
}

func TestGetLogsClampsAnEnormousLimit(t *testing.T) {
	seedLogs(t, "ClampHost", "ClampCont", 1100)

	logs := GetLogs(false, false, "ClampHost", "ClampCont", "", 1<<30, "", false, nil)["logs"].([][]string)
	if len(logs) > maxLogsPerRequest {
		t.Fatalf("a caller-supplied limit of 2^30 returned %d rows; the response is unbounded", len(logs))
	}
}

func TestGetLogsResumesPastTheScanCap(t *testing.T) {
	seedLogs(t, "CapHost", "CapCont", 60)

	original := maxScanIterations
	maxScanIterations = 10
	t.Cleanup(func() { maxScanIterations = original })

	seen := map[string]int{}
	cursor := ""
	for round := 0; round < 40; round++ {
		// "line 0" matches exactly one row, the OLDEST — far beyond the first cap
		// window, since the scan walks newest-first.
		result := GetLogs(false, false, "CapHost", "CapCont", "line 0", 30, cursor, false, nil)

		for _, row := range result["logs"].([][]string) {
			seen[row[1]]++
		}
		if result["is_end"] == true {
			break
		}

		next := result["last_processed_key"].(string)
		if next == "" {
			t.Fatalf("round %d: a capped scan returned no cursor, so the client restarts at the newest row", round)
		}
		if next == cursor {
			t.Fatalf("round %d: the cursor did not advance past the cap (%q)", round, next)
		}
		cursor = next
	}

	if len(seen) == 0 {
		t.Fatal("a rare search term past the scan cap was never reachable")
	}
	for message, count := range seen {
		if count != 1 {
			t.Errorf("row %q was returned %d times across the capped scan", message, count)
		}
	}
}

// GetDB opens-or-creates and caches forever, so an unknown container name must
// not reach it: each one would allocate a LevelDB tree and pin its descriptors.
func TestGetLogsDoesNotCreateADatabaseForAnUnknownContainer(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/RealHost/containers/realcontainer", 0o700); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 5; i++ {
		name := "no-such-container-" + strconv.Itoa(i)
		result := GetLogs(false, false, "RealHost", name, "", 30, "", false, nil)
		if rows := result["logs"].([][]string); len(rows) != 0 {
			t.Fatalf("%s returned %d rows", name, len(rows))
		}
	}
	GetLogs(false, false, "NoSuchHostAtAll", "whatever", "", 30, "", false, nil)

	entries, err := os.ReadDir("leveldb/hosts/RealHost/containers")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "realcontainer" {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("reading unknown containers created databases: %v", names)
	}
	if _, err := os.Stat("leveldb/hosts/NoSuchHostAtAll"); err == nil {
		t.Error("reading an unknown host created a host directory")
	}
}
