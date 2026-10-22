package statistics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestStatsCountRowsStoredAtOrBehindTheCursorAfterAFlush(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset time.Duration
	}{
		{"Equal", 0},
		{"Older", -time.Second},
		{"Newer", time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, container := "LateRow"+tc.name, "svc"
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

			put := func(at time.Time) {
				t.Helper()
				line := []string{at.UTC().Format(fixedWidthNanos), "ERROR " + at.String()}
				if err := containerdb.PutLogMessage(util.GetDB(host, container, "logs"), host, container, line); err != nil {
					t.Fatalf("put: %v", err)
				}
			}
			check := func(step string, want uint64) {
				t.Helper()
				service := GetStatisticsByService(host, container, 48)["error"]
				var chart uint64
				for _, levels := range GetChartData(host, container, "hour", 24) {
					chart += levels["error"]
				}
				if service != want || chart != want {
					t.Errorf("%s: %d rows stored, service=%d chart=%d", step, want, service, chart)
				}
			}

			stamp := time.Now().UTC().Add(-time.Minute)
			put(stamp)
			restartStats(host, container)
			put(stamp.Add(tc.offset))
			restartStats(host, container)
			check("after a flush", 2)

			restartStats(host, container)
			put(stamp.Add(2 * time.Second))
			restartStats(host, container)
			check("after a newer row", 3)

			for _, dbType := range []string{"logs", "statuses", "statistics"} {
				util.ResetDB(host, container, dbType)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			RunStatisticForContainerWithContext(ctx, host, container)
			check("after a worker restart", 3)
		})
	}
}

func TestOverlappingFlushesFromOneCursorCountOnce(t *testing.T) {
	host, container := "OverlapFlush", "svc"
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

	put := func(at time.Time) {
		line := []string{at.Format(fixedWidthNanos), "ERROR " + at.String()}
		if err := containerdb.PutLogMessage(util.GetDB(host, container, "logs"), host, container, line); err != nil {
			t.Fatalf("put: %v", err)
		}
	}
	stamp := time.Now().UTC().Add(-time.Minute)
	put(stamp)
	restartStats(host, container)
	put(stamp.Add(time.Second))

	db := util.GetDB(host, container, "statistics")
	cursor := getLastStatTime(db)
	for range 2 {
		hours, _ := collectLogs(host, container, cursor, true)
		batch := new(leveldb.Batch)
		saveHours(batch, hours)
		db.Write(batch, nil)
	}
	resetInMemoryStats(host + "/" + container)

	if got := GetStatisticsByService(host, container, 48)["error"]; got != 2 {
		t.Errorf("2 rows stored, got %d", got)
	}
}

func TestAFlushStartedDuringAnotherCountsALateRowOnce(t *testing.T) {
	host, container := "LateRowOverlap", "svc"
	location := host + "/" + container
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		SetBeforeStatsSave(nil)
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, container, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, location)
		vars.Mutex.Unlock()
	})

	put := func(at time.Time) {
		line := []string{at.Format(fixedWidthNanos), "ERROR " + at.String()}
		if err := containerdb.PutLogMessage(util.GetDB(host, container, "logs"), host, container, line); err != nil {
			t.Errorf("put: %v", err)
		}
	}
	stamp := time.Now().UTC().Add(-time.Minute)
	put(stamp)
	restartStats(host, container)
	put(stamp.Add(2 * time.Second))

	fired, second := false, make(chan struct{})
	SetBeforeStatsSave(func(l string) {
		if l != location || fired {
			return
		}
		fired = true
		put(stamp.Add(time.Second))
		go func() {
			restartStats(host, container)
			close(second)
		}()
		time.Sleep(100 * time.Millisecond)
	})
	restartStats(host, container)
	<-second

	service := GetStatisticsByService(host, container, 48)["error"]
	var chart uint64
	for _, levels := range GetChartData(host, container, "hour", 24) {
		chart += levels["error"]
	}
	if service != 3 || chart != 3 {
		t.Errorf("3 rows stored, service=%d chart=%d", service, chart)
	}
}

func TestRowsStoredWhileAFlushScansAreCountedOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		first  bool
		offset time.Duration
	}{
		{"Equal", false, 0},
		{"Older", false, -500 * time.Millisecond},
		{"Newer", false, time.Second},
		{"FirstFlushEqual", true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host, container := "ScanWindow"+tc.name, "svc"
			location := host + "/" + container
			os.RemoveAll("leveldb/hosts/" + host)
			t.Cleanup(func() {
				SetAfterStatsScan(nil)
				for _, dbType := range []string{"logs", "statuses", "statistics"} {
					util.ResetDB(host, container, dbType)
				}
				os.RemoveAll("leveldb/hosts/" + host)
				vars.Mutex.Lock()
				delete(vars.Container_Stat_Counter, location)
				vars.Mutex.Unlock()
			})

			put := func(at time.Time) {
				line := []string{at.Format(fixedWidthNanos), "ERROR " + at.String()}
				if err := containerdb.PutLogMessage(util.GetDB(host, container, "logs"), host, container, line); err != nil {
					t.Errorf("put: %v", err)
				}
			}
			stamp, want := time.Now().UTC().Add(-time.Minute), uint64(2)
			put(stamp)
			if !tc.first {
				restartStats(host, container)
				stamp, want = stamp.Add(time.Second), 3
				put(stamp)
			}

			fired, stored := false, make(chan struct{})
			SetAfterStatsScan(func(l string) {
				if l != location || fired {
					return
				}
				fired = true
				go func() {
					put(stamp.Add(tc.offset))
					close(stored)
				}()
				select {
				case <-stored:
				case <-time.After(100 * time.Millisecond):
				}
			})
			restartStats(host, container)
			SetAfterStatsScan(nil)
			if !fired {
				t.Fatal("the flush never reached the hook")
			}
			<-stored
			restartStats(host, container)
			restartStats(host, container)

			service := GetStatisticsByService(host, container, 48)["error"]
			var chart uint64
			for _, levels := range GetChartData(host, container, "hour", 24) {
				chart += levels["error"]
			}
			if service != want || chart != want {
				t.Errorf("%d rows stored, service=%d chart=%d", want, service, chart)
			}
		})
	}
}
