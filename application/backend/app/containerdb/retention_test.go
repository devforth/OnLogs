package containerdb

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

type retentionSeed struct {
	container string
	rows      int
	start     time.Duration
}

var retentionBase = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

func holdCleanup(t *testing.T) {
	t.Chdir(t.TempDir())
	logCleanupMu.Lock()
	saved := nextCleanup
	nextCleanup = time.Now().Add(time.Hour)
	logCleanupMu.Unlock()
	t.Cleanup(func() {
		logCleanupMu.Lock()
		nextCleanup = saved
		logCleanupMu.Unlock()
	})
	for running := true; running; time.Sleep(time.Millisecond) {
		logCleanupMu.Lock()
		running = isCleanupRunning
		logCleanupMu.Unlock()
	}
}

func seedRetentionRows(t *testing.T, host string, seeds []retentionSeed) {
	r := rand.New(rand.NewSource(1))
	text := make([]byte, 4096)
	for _, s := range seeds {
		db := util.GetDB(host, s.container, "logs")
		if db == nil {
			t.Fatal("could not open the logs database")
		}
		t.Cleanup(func() { DeleteContainer(host, s.container, true) })
		for i := 0; i < s.rows; i++ {
			for j := range text {
				text[j] = 'a' + byte(r.Intn(26))
			}
			ts := retentionBase.Add(s.start + time.Duration(i)*time.Second).Format("2006-01-02T15:04:05.000000000Z")
			if err := PutLogMessage(db, host, s.container, []string{ts, "ERROR " + string(text)}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func prunableBytes(host string, containers ...string) (n int64) {
	for _, c := range containers {
		n += int64(util.GetPrunableSize(host, c) * 1024 * 1024)
	}
	return n
}

func logKeys(host, container string) (ks []string) {
	iter := util.GetDB(host, container, "logs").NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		ks = append(ks, string(iter.Key()))
	}
	return ks
}

func assertRetentionReachesQuota(t *testing.T, host string, seeds []retentionSeed, percent int64) {
	holdCleanup(t)
	seedRetentionRows(t, host, seeds)

	var containers []string
	before := map[string][]string{}
	for _, s := range seeds {
		containers = append(containers, s.container)
		before[s.container] = logKeys(host, s.container)
	}
	limit := prunableBytes(host, containers...) * percent / 100
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")

	if err := checkAndManageLogSize(host, seeds[0].container); err != nil {
		t.Fatal(err)
	}

	if size := prunableBytes(host, containers...); size > limit {
		t.Fatalf("cleanup returned with %d bytes over the %d byte limit", size, limit)
	}
	kept := 0
	for c, ks := range before {
		after := logKeys(host, c)
		kept += len(after)
		if !slices.Equal(after, ks[len(ks)-len(after):]) {
			t.Errorf("%s lost a row newer than one it kept", c)
		}
	}
	if kept == 0 {
		t.Error("cleanup emptied the store although deleting part of it fits the limit")
	}
}

func TestRetentionTrimsStoreWhereEveryContainerHoldsFewerThan200Rows(t *testing.T) {
	var seeds []retentionSeed
	for k := 0; k < 10; k++ {
		seeds = append(seeds, retentionSeed{fmt.Sprintf("small-%02d", k), 50, 0})
	}
	assertRetentionReachesQuota(t, "UnderBatchHost", seeds, 50)
}

func TestRetentionKeepsTrimmingOnceLargeContainersDropBelow200Rows(t *testing.T) {
	seeds := []retentionSeed{{"small-00", 150, 0}}
	for k := 0; k < 6; k++ {
		seeds = append(seeds, retentionSeed{fmt.Sprintf("big-%d", k), 250, 1000 * time.Second})
	}
	assertRetentionReachesQuota(t, "MixedBatchHost", seeds, 15)
}

func TestRetentionDoesNotSweepOlderSmallContainersOnceANewerOneHolds200Rows(t *testing.T) {
	for _, rows := range []int{50, 100} {
		t.Run(strconv.Itoa(rows), func(t *testing.T) {
			var seeds []retentionSeed
			for k := 0; k < 3; k++ {
				seeds = append(seeds, retentionSeed{fmt.Sprintf("small-%d", k), rows, time.Duration(k*rows) * time.Second})
			}
			seeds = append(seeds, retentionSeed{"onlogs", 200, time.Hour})
			assertRetentionReachesQuota(t, "SweepHost"+strconv.Itoa(rows), seeds, 70)
		})
	}
}

func TestRetentionDeletesWhenTheNewestRowHasAnUnparseableTimestamp(t *testing.T) {
	holdCleanup(t)
	host, c := "JunkTimeHost", "junk"
	seedRetentionRows(t, host, []retentionSeed{{c, 20, 0}})
	if err := PutLogMessage(util.GetDB(host, c, "logs"), host, c, []string{strings.Repeat("z", 30), "INFO junk"}); err != nil {
		t.Fatal(err)
	}
	limit := prunableBytes(host, c) / 2
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")

	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatal(err)
	}
	if size := prunableBytes(host, c); size > limit {
		t.Errorf("cleanup returned with %d bytes over the %d byte limit, %d rows left", size, limit, len(logKeys(host, c)))
	}
}

func TestRetentionCheckEndsAfterDeletingEveryRowWhileItsOutputIsIngested(t *testing.T) {
	holdCleanup(t)
	host, self := "SelfIngestHost", "onlogs"
	seedRetentionRows(t, host, []retentionSeed{{"app-0", 3, 0}, {"app-1", 3, time.Minute}, {"app-2", 3, 2 * time.Minute}})
	t.Cleanup(func() { DeleteContainer(host, self, true) })
	t.Setenv("MAX_LOGS_SIZE", "1B")

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	ingested := make(chan struct{})
	go func() {
		defer close(ingested)
		sc := bufio.NewScanner(pr)
		for n := 1; sc.Scan(); n++ {
			if n <= 20 {
				ts := retentionBase.Add(time.Hour + time.Duration(n)*time.Second).Format("2006-01-02T15:04:05.000000000Z")
				_ = PutLogMessage(util.GetDB(host, self, "logs"), host, self, []string{ts, sc.Text()})
			}
		}
	}()
	stdout := os.Stdout
	os.Stdout = pw
	before := RetentionDeletedLines()
	err = checkAndManageLogSize(host, self)
	deleted := RetentionDeletedLines() - before
	pw.Close()
	<-ingested
	os.Stdout = stdout
	pr.Close()

	if err != nil {
		t.Fatal(err)
	}
	if deleted != 9 {
		t.Errorf("deleted %d rows, want only the 9 stored when the check started", deleted)
	}
}

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

func TestRetentionKeepsLogsWhileDeletableDataFitsTheQuota(t *testing.T) {
	t.Chdir(t.TempDir())
	host, container := "QuotaHost", "QuotaContainer"

	logCleanupMu.Lock()
	saved := nextCleanup
	nextCleanup = time.Now().Add(time.Hour)
	logCleanupMu.Unlock()
	t.Cleanup(func() {
		logCleanupMu.Lock()
		nextCleanup = saved
		logCleanupMu.Unlock()
	})

	logsDB := util.GetDB(host, container, "logs")
	if logsDB == nil {
		t.Fatal("could not open the logs database")
	}
	t.Cleanup(func() { DeleteContainer(host, container, true) })
	base := time.Date(2026, 2, 10, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 250; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format("2006-01-02T15:04:05.000000000Z")
		if err := PutLogMessage(logsDB, host, container, []string{ts, fmt.Sprintf("ERROR line %d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	dir := "leveldb/hosts/" + host + "/containers/" + container
	if err := os.MkdirAll(dir+"/brokenlogs", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/brokenlogs/ballast", make([]byte, 8*1024*1024), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MAX_LOGS_SIZE", "4MB")
	if prunable := util.GetPrunableSize(host, container); prunable >= 4 {
		t.Fatalf("precondition: prunable data is %.2f MiB, not under the 4MB limit", prunable)
	}
	if err := checkAndManageLogSize(host, container); err != nil {
		t.Fatal(err)
	}

	rows := 0
	iter := logsDB.NewIterator(nil, nil)
	for iter.Next() {
		rows++
	}
	iter.Release()
	if rows != 250 {
		t.Errorf("retention deleted %d rows to offset data it cannot delete", 250-rows)
	}
}

func TestRetentionRunsAPendingCheckWhenTheCooldownExpiresWithoutAnotherLine(t *testing.T) {
	holdCleanup(t)
	host, c := "IdleHost", "idle"
	const limit = 256 * 1024
	t.Setenv("MAX_LOGS_SIZE", strconv.Itoa(limit)+"B")

	logCleanupMu.Lock()
	savedCooldown := cleanupCooldown
	cleanupCooldown = time.Second
	nextCleanup = time.Time{}
	logCleanupMu.Unlock()
	t.Cleanup(func() {
		logCleanupMu.Lock()
		cleanupCooldown = savedCooldown
		logCleanupMu.Unlock()
	})
	waitFor := func(timeout time.Duration, cond func() bool) bool {
		for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			if cond() {
				return true
			}
		}
		return false
	}
	var expiry time.Time
	checkDone := func() bool {
		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()
		expiry = nextCleanup
		return !isCleanupRunning && !nextCleanup.IsZero()
	}

	seedRetentionRows(t, host, []retentionSeed{{c, 1, 0}})
	if !waitFor(10*time.Second, checkDone) {
		t.Fatal("the first line's check did not finish")
	}
	before := RetentionDeletedLines()
	seedRetentionRows(t, host, []retentionSeed{{c, 250, time.Second}})
	if !time.Now().Before(expiry) || RetentionDeletedLines() != before || prunableBytes(host, c) <= limit {
		t.Fatal("precondition: the quota was not crossed inside the cooldown with nothing deleted")
	}

	if !waitFor(10*time.Second, func() bool { return RetentionDeletedLines() > before }) {
		t.Fatal("no check ran after the cooldown expired while the idle store was over the limit")
	}
	if time.Now().Before(expiry) {
		t.Error("the pending check ran before the cooldown expired")
	}
	if !waitFor(10*time.Second, checkDone) {
		t.Fatal("the pending check did not finish")
	}
	if size := prunableBytes(host, c); size > limit {
		t.Errorf("the idle store is still %d bytes against a %d byte limit", size, limit)
	}
}
