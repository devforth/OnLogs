package containerdb

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
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
	for _, s := range seeds {
		at := make([]time.Duration, s.rows)
		for i := range at {
			at[i] = s.start + time.Duration(i)*time.Second
		}
		seedRowsAt(t, r, host, s.container, at)
	}
}

func seedRowsAt(t *testing.T, r *rand.Rand, host, container string, at []time.Duration) {
	db := util.GetDB(host, container, "logs")
	if db == nil {
		t.Fatal("could not open the logs database")
	}
	t.Cleanup(func() { DeleteContainer(host, container, true) })
	text := make([]byte, 4096)
	for _, d := range at {
		for j := range text {
			text[j] = 'a' + byte(r.Intn(26))
		}
		ts := retentionBase.Add(d).Format("2006-01-02T15:04:05.000000000Z")
		if err := PutLogMessage(db, host, container, []string{ts, "ERROR " + string(text)}); err != nil {
			t.Fatal(err)
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
	t.Cleanup(func() {
		DeleteContainer(host, container, true)
		_ = os.RemoveAll("leveldb/hosts/" + host)
	})

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

func assertSurvivorsKeepTheirSeverity(t *testing.T, host, c string) {
	t.Helper()
	statuses := util.GetDB(host, c, "statuses")
	rows, missing := logKeys(host, c), 0
	for _, k := range rows {
		if _, err := statuses.Get([]byte(k), nil); err != nil {
			missing++
		}
	}
	level := "error"
	all := GetLogs(true, false, host, c, "", maxLogsPerRequest, "", false, nil)["logs"].([][]string)
	filtered := GetLogs(true, false, host, c, "", maxLogsPerRequest, "", false, &level)["logs"].([][]string)
	if missing > 0 || len(filtered) != len(all) {
		t.Errorf("%d of %d surviving rows lost their severity entry; the error filter returns %d", missing, len(all), len(filtered))
	}
	entries := 0
	iter := statuses.NewIterator(nil, nil)
	for iter.Next() {
		entries++
	}
	iter.Release()
	if entries != len(rows)-missing {
		t.Errorf("statuses holds %d entries for %d rows: deleted rows kept theirs", entries, len(rows)-missing)
	}
}

func TestRetentionKeepsSeverityOfRowsTiedWithTheCutoff(t *testing.T) {
	boundary, same := make([]time.Duration, 250), make([]time.Duration, 250)
	for i := range boundary {
		boundary[i] = time.Duration(i) * time.Second
	}
	boundary[200] = boundary[199]
	for _, tc := range []struct {
		name string
		at   []time.Duration
	}{
		{"201st row shares the 200th's timestamp", boundary},
		{"every row shares one timestamp", same},
	} {
		t.Run(tc.name, func(t *testing.T) {
			holdCleanup(t)
			host, c := "TieHost", "tie"
			seedRowsAt(t, rand.New(rand.NewSource(1)), host, c, tc.at)
			t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)*9/10, 10)+"B")

			before := RetentionDeletedLines()
			if err := checkAndManageLogSize(host, c); err != nil {
				t.Fatal(err)
			}
			if deleted := RetentionDeletedLines() - before; deleted != 200 {
				t.Fatalf("deleted %d rows, want one round of 200", deleted)
			}
			assertSurvivorsKeepTheirSeverity(t, host, c)
		})
	}
}

func TestRetentionKeepsSeverityOfOlderRowsStoredWhileItRuns(t *testing.T) {
	for _, stage := range []string{"logs", "statuses"} {
		t.Run("stored before the "+stage+" prune", func(t *testing.T) {
			holdCleanup(t)
			host, c := "DelayedHost", "delayed"
			seedRetentionRows(t, host, []retentionSeed{{c, 400, 0}})
			t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)*9/10, 10)+"B")

			stored := false
			saved := beforePrune
			t.Cleanup(func() { beforePrune = saved })
			beforePrune = func(h, ctr, dbType string) {
				if dbType == stage && !stored {
					stored = true
					seedRetentionRows(t, host, []retentionSeed{{c, 16, -time.Hour}})
				}
			}

			if err := checkAndManageLogSize(host, c); err != nil {
				t.Fatal(err)
			}
			if !stored {
				t.Fatal("cleanup never reached the step under test")
			}
			assertSurvivorsKeepTheirSeverity(t, host, c)
		})
	}
}

func TestRetentionPrunesAbandonedHostsFirstAndRemovesThemOnceEmpty(t *testing.T) {
	holdCleanup(t)
	local, old, unused, agent := util.GetHost(), "0123456789ab", "fedcba987654", "agent-host"
	seedRetentionRows(t, old, []retentionSeed{{"app", 300, 0}})
	seedRetentionRows(t, local, []retentionSeed{{"app", 600, time.Hour}})
	for _, h := range []string{unused, agent} {
		if err := os.MkdirAll("leveldb/hosts/"+h+"/containers/c", 0o700); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { DeleteContainer(h, "c", true) })
	}
	vars.SetAgentContainers(agent, []string{"c"})
	t.Cleanup(func() { vars.SetAgentContainers(agent, nil) })

	newest := logKeys(local, "app")
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(local, "app"), 10)+"B")
	if err := checkAndManageLogSize(local, "app"); err != nil {
		t.Fatal(err)
	}

	for _, h := range []string{old, unused} {
		if _, err := os.Stat("leveldb/hosts/" + h); !os.IsNotExist(err) {
			t.Errorf("host %s holds no rows but is still there (%v)", h, err)
		}
	}
	for _, h := range []string{local, agent} {
		if _, err := os.Stat("leveldb/hosts/" + h); err != nil {
			t.Errorf("host %s was removed: %v", h, err)
		}
	}
	kept := logKeys(local, "app")
	if !slices.Equal(kept, newest[len(newest)-len(kept):]) {
		t.Error("the current host lost a row newer than one it kept")
	}
	if lost := len(newest) - len(kept); lost >= 300 {
		t.Errorf("the current host lost %d rows, more than the abandoned host held", lost)
	}

	t.Setenv("MAX_LOGS_SIZE", "1B")
	if err := checkAndManageLogSize(local, "app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("leveldb/hosts/" + local + "/containers/app"); err != nil {
		t.Errorf("the current host was removed once retention emptied it: %v", err)
	} else if rows := len(logKeys(local, "app")); rows > 0 {
		t.Fatalf("the current host still holds %d rows", rows)
	}
}

func TestRetentionKeepsAnOldHostWhoseLogsCannotBeRead(t *testing.T) {
	holdCleanup(t)
	local, old := util.GetHost(), "0123456789ab"
	seedRetentionRows(t, old, []retentionSeed{{"app", 50, 0}})
	seedRetentionRows(t, local, []retentionSeed{{"app", 100, time.Hour}})
	if err := util.GetDB(old, "app", "logs").CompactRange(leveldbUtil.Range{}); err != nil {
		t.Fatal(err)
	}
	util.ResetDB(old, "app", "logs")
	tables, _ := filepath.Glob("leveldb/hosts/" + old + "/containers/app/logs/*.ldb")
	if len(tables) == 0 {
		t.Fatal("no table to damage")
	}
	for _, f := range tables {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := range len(b) / 2 {
			b[i] ^= 0xff
		}
		if err := os.WriteFile(f, b, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(local, "app"), 10)+"B")
	if err := checkAndManageLogSize(local, "app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("leveldb/hosts/" + old + "/containers/app/logs"); err != nil {
		t.Errorf("a host whose rows could not be read was removed as empty: %v", err)
	}
}
