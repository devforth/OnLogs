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
		ts := vars.Year + "-02-10T12:" + pad(i/60) + ":" + pad(i%60) + ".230421754Z"
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

func TestGetLogsReportsEndWhenTheScanCapStopsIt(t *testing.T) {
	seedLogs(t, "CapHost", "CapCont", 50)

	original := maxScanIterations
	maxScanIterations = 10
	t.Cleanup(func() { maxScanIterations = original })

	result := GetLogs(false, false, "CapHost", "CapCont", "no-such-text-anywhere", 30, "", false, nil)

	if result["is_end"] != true {
		t.Fatalf("the scan stopped at the iteration cap but reported is_end=%v; the client re-requests the same page forever", result["is_end"])
	}
}
