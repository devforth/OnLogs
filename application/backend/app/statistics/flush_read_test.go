package statistics_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

var statsAPIs = []string{"getStats", "getChartData"}

func newStatsContainer(t *testing.T, name string) (string, string) {
	t.Helper()
	t.Setenv("DISABLE_AUTH", "true")
	host, svc := "FlushRead"+name, "svc"
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, svc, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, host+"/"+svc)
		vars.Mutex.Unlock()
	})
	if util.GetDB(host, svc, "statistics") == nil {
		t.Fatal("no statistics database")
	}
	return host, svc
}

func containerWith32UnflushedErrors(t *testing.T, name string) (string, string) {
	t.Helper()
	host, svc := newStatsContainer(t, name)
	logs := util.GetDB(host, svc, "logs")
	put := func(at time.Time, line string) {
		if err := containerdb.PutLogMessage(logs, host, svc, []string{at.Format("2006-01-02T15:04:05.000000000Z"), line}); err != nil {
			t.Fatal(err)
		}
	}
	at := time.Now().UTC().Add(-time.Minute)
	put(at, "ONLOGS: listening started")
	statistics.RestartStats(host, svc)
	for i := 1; i <= 32; i++ {
		put(at.Add(time.Duration(i)*time.Millisecond), "ERROR boom")
	}
	for _, api := range statsAPIs {
		if got := errorsFrom(t, api, host, svc); got != 32 {
			t.Fatalf("before the flush %s returned %d errors, want 32", api, got)
		}
	}
	return host, svc
}

// Safe off the test goroutine: it never calls t.Fatal.
func errorsFrom(t *testing.T, api, host, svc string) uint64 {
	t.Helper()
	ctrl := &routes.RouteController{}
	handler, body := ctrl.GetStats, map[string]any{"host": host, "service": svc, "period": 48}
	if api == "getChartData" {
		handler, body = ctrl.GetChartData, map[string]any{"host": host, "service": svc, "unit": "hour", "unitsAmount": 24}
	}
	raw, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodPost, "/api/v1/"+api, bytes.NewReader(raw)))
	if rr.Code != http.StatusOK {
		t.Errorf("%s returned %d: %s", api, rr.Code, rr.Body)
		return 0
	}

	if api == "getStats" {
		var stats map[string]uint64
		if err := json.Unmarshal(rr.Body.Bytes(), &stats); err != nil {
			t.Errorf("%s: %v", api, err)
		}
		return stats["error"]
	}
	var chart map[string]map[string]uint64
	if err := json.Unmarshal(rr.Body.Bytes(), &chart); err != nil {
		t.Errorf("%s: %v", api, err)
	}
	var total uint64
	for _, levels := range chart {
		total += levels["error"]
	}
	return total
}

func TestStatsEndpointsCountEachLineOnceWhenAReadSpansAFlush(t *testing.T) {
	for _, api := range statsAPIs {
		t.Run(api, func(t *testing.T) {
			host, svc := containerWith32UnflushedErrors(t, "Span"+api)
			flushed := false
			statistics.SetAfterStatsRead(func() {
				if !flushed {
					flushed = true
					statistics.RestartStats(host, svc)
				}
			})
			t.Cleanup(func() { statistics.SetAfterStatsRead(nil) })

			got := errorsFrom(t, api, host, svc)
			statistics.SetAfterStatsRead(nil)
			if !flushed {
				t.Fatal("the read never reached the hook")
			}
			if got != 32 {
				t.Errorf("%s returned %d errors across a flush, want 32", api, got)
			}
			for _, after := range statsAPIs {
				if got := errorsFrom(t, after, host, svc); got != 32 {
					t.Errorf("after the flush %s returned %d errors, want 32", after, got)
				}
			}
		})
	}
}

func TestStatsEndpointsCountEachLineOnceWhenReadBetweenSaveAndReset(t *testing.T) {
	for _, api := range statsAPIs {
		t.Run(api, func(t *testing.T) {
			host, svc := containerWith32UnflushedErrors(t, "Gap"+api)
			result := make(chan uint64, 1)
			fired := false
			statistics.SetBeforeStatsReset(func(location string) {
				if location != host+"/"+svc {
					return
				}
				fired = true
				go func() { result <- errorsFrom(t, api, host, svc) }()
				select {
				case got := <-result:
					result <- got
				case <-time.After(100 * time.Millisecond):
				}
			})
			t.Cleanup(func() { statistics.SetBeforeStatsReset(nil) })

			statistics.RestartStats(host, svc)
			statistics.SetBeforeStatsReset(nil)
			if !fired {
				t.Fatal("the flush never reached the hook")
			}
			if got := <-result; got != 32 {
				t.Errorf("%s returned %d errors between a flush's save and reset, want 32", api, got)
			}
		})
	}
}

func TestStatsEndpointsKeepTheirTimeWindow(t *testing.T) {
	host, svc := newStatsContainer(t, "Window")
	db := util.GetDB(host, svc, "statistics")
	synctest.Test(t, func(t *testing.T) {
		cutoff := time.Now().UTC().Add(-24 * time.Hour)
		records := map[time.Duration]string{time.Nanosecond: `{"error":1}`, 0: `{"error":10}`, -time.Nanosecond: `{"error":100}`}
		for offset, record := range records {
			if err := db.Put([]byte(cutoff.Add(offset).Format(time.RFC3339Nano)), []byte(record), nil); err != nil {
				t.Fatal(err)
			}
		}
		vars.Mutex.Lock()
		vars.Container_Stat_Counter[host+"/"+svc] = map[string]uint64{"error": 1000}
		vars.Mutex.Unlock()

		for _, api := range statsAPIs {
			if got := errorsFrom(t, api, host, svc); got != 1011 {
				t.Errorf("%s returned %d errors, want 1011 (live plus the records just inside and on the cutoff)", api, got)
			}
		}
	})
}
