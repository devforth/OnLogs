package containerdb

import (
	"bufio"
	"math/rand"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
)

func registerDB(t *testing.T, host, container, dbType string, stor storage.Storage, o *opt.Options) *leveldb.DB {
	t.Helper()
	db, err := leveldb.Open(stor, o)
	if err != nil {
		t.Fatal(err)
	}
	cache := map[string]map[string]*leveldb.DB{
		"logs": vars.ActiveDBs, "statuses": vars.Statuses_DBs, "statistics": vars.Stat_Containers_DBs,
	}[dbType]
	vars.DBMutex.Lock()
	cache[host+"/"+container] = db
	vars.DBMutex.Unlock()
	t.Cleanup(func() {
		util.ResetDB(host, container, dbType)
		stor.Close()
	})
	return db
}

func openStorage(t *testing.T, host, container, dbType string) storage.Storage {
	t.Helper()
	stor, err := storage.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/"+dbType, false)
	if err != nil {
		t.Fatal(err)
	}
	return stor
}

func ioWrites(dbs ...*leveldb.DB) (n uint64) {
	for _, db := range dbs {
		var s leveldb.DBStats
		if db.Stats(&s) == nil {
			n += s.IOWrite
		}
	}
	return n
}

func countRounds(t *testing.T, also func(round int)) *int {
	rounds := 0
	saved := beforePrune
	t.Cleanup(func() { beforePrune = saved })
	beforePrune = func(h, c, dbType string) {
		if dbType == "logs" {
			rounds++
			if also != nil {
				also(rounds)
			}
		}
	}
	return &rounds
}

func settle(t *testing.T, db *leveldb.DB, o *opt.Options) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		var s leveldb.DBStats
		if err := db.Stats(&s); err != nil {
			t.Fatal(err)
		}
		settled := len(s.LevelTablesCounts) == 0 || s.LevelTablesCounts[0] < o.GetCompactionL0Trigger()
		for level := 1; level < len(s.LevelSizes); level++ {
			settled = settled && s.LevelSizes[level] < o.GetCompactionTotalSize(level)
		}
		if settled {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("compaction did not settle: %v", s.LevelSizes)
		}
	}
}

func compactAndSettle(t *testing.T, r leveldbUtil.Range, o *opt.Options, dbs ...*leveldb.DB) {
	t.Helper()
	for _, db := range dbs {
		if err := db.CompactRange(r); err != nil {
			t.Fatal(err)
		}
		settle(t, db, o)
	}
}

func TestRetentionRewritesOnlyWhatItDeletesWhileLinesArrive(t *testing.T) {
	holdCleanup(t)
	host, c := "RewriteHost", "app"
	o := *util.DBOptions
	o.CompactionTableSize, o.CompactionTotalSize = 32<<10, 32<<10
	logs := registerDB(t, host, c, "logs", openStorage(t, host, c, "logs"), &o)
	statuses := registerDB(t, host, c, "statuses", openStorage(t, host, c, "statuses"), &o)
	seedRetentionRows(t, host, []retentionSeed{{c, 1200, 0}})
	compactAndSettle(t, leveldbUtil.Range{}, &o, logs, statuses)
	newer := retentionBase.Add(time.Hour)
	seedRetentionRows(t, host, []retentionSeed{{c, 300, time.Hour}})
	compactAndSettle(t, leveldbUtil.Range{Start: []byte(newer.Format(time.RFC3339))}, &o, logs, statuses)
	before := prunableBytes(host, c)
	limit := before * 97 / 100
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")

	var written []uint64
	countRounds(t, func(round int) {
		written = append(written, ioWrites(logs, statuses))
		ts := newer.Add(time.Hour + time.Duration(round)*time.Second).Format("2006-01-02T15:04:05.000000000Z")
		if err := PutLogMessage(logs, host, c, []string{ts, "INFO fresh"}); err != nil {
			t.Error(err)
		}
	})
	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatal(err)
	}
	written = append(written, ioWrites(logs, statuses))
	for round := 1; round < len(written); round++ {
		if got := written[round] - written[round-1]; got > uint64(before/20) {
			t.Errorf("round %d wrote %d bytes, over 5%% of the %d byte store", round, got, before)
		}
	}
	if size := prunableBytes(host, c); size > limit {
		t.Errorf("cleanup returned with %d bytes over the %d byte limit", size, limit)
	}
}

func TestRetentionDoesNotDeleteMostOfTheStoreWhenLinesArriveDuringARound(t *testing.T) {
	holdCleanup(t)
	host, c := "BurstHost", "app"
	seedRetentionRows(t, host, []retentionSeed{{c, 2000, 0}})
	for _, dbType := range []string{"logs", "statuses"} {
		db := util.GetDB(host, c, dbType)
		if err := db.CompactRange(leveldbUtil.Range{}); err != nil {
			t.Fatal(err)
		}
		settle(t, db, util.DBOptions)
	}
	limit := prunableBytes(host, c) - 100<<10
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")
	const arrivals = 198
	countRounds(t, func(round int) {
		if round == 1 {
			at := make([]time.Duration, arrivals)
			for i := range at {
				at[i] = time.Hour + time.Duration(i)*time.Second
			}
			seedRowsAt(t, rand.New(rand.NewSource(7)), host, c, at)
		}
	})
	before := len(logKeys(host, c))
	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatal(err)
	}
	if deleted := before + arrivals - len(logKeys(host, c)); deleted > 2*(arrivals+minRoundRows) {
		t.Errorf("deleted %d rows to remove an excess of about %d", deleted, arrivals+25)
	}
}

func TestRetentionKeepsNoSeverityOfDeletedRowsWhenTheDiskFillsMidRun(t *testing.T) {
	holdCleanup(t)
	host, c := "SeverityRoomHost", "app"
	seedRetentionRows(t, host, []retentionSeed{{c, 300, 0}})
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)/2, 10)+"B")
	before := logKeys(host, c)

	var checks atomic.Int32
	saved := util.FreeDiskBytes
	t.Cleanup(func() { util.FreeDiskBytes = saved })
	util.FreeDiskBytes = func() (uint64, error) {
		if checks.Add(1) == 1 {
			return 1 << 40, nil
		}
		return 0, nil
	}
	checkAndManageLogSize(host, c)
	util.FreeDiskBytes = saved
	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatal(err)
	}

	after := logKeys(host, c)
	statuses := util.GetDB(host, c, "statuses")
	for _, key := range before[:len(before)-len(after)] {
		if ok, _ := statuses.Has([]byte(key), nil); ok {
			t.Fatalf("deleted row %s kept its severity entry", key)
		}
	}
}

func TestRetentionSizesEachRoundFromTheExcess(t *testing.T) {
	holdCleanup(t)
	host, c := "ExcessHost", "app"
	seedRetentionRows(t, host, []retentionSeed{{c, 1000, 0}})
	limit := prunableBytes(host, c) / 5
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")

	rounds := countRounds(t, nil)
	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatal(err)
	}
	if *rounds > 3 {
		t.Errorf("took %d rounds to trim the store to a fifth", *rounds)
	}
	if size := prunableBytes(host, c); size > limit {
		t.Errorf("cleanup returned with %d bytes over the %d byte limit", size, limit)
	}
}

func TestRetentionStopsReadingStatisticsAtTheFirstNewerEntry(t *testing.T) {
	holdCleanup(t)
	host, c := "StatsScanHost", "app"
	seedRetentionRows(t, host, []retentionSeed{{c, 250, 0}})
	stats := util.GetDB(host, c, "statistics")
	key := func(i int) []byte {
		return []byte(retentionBase.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano))
	}
	for i := 0; i < 250; i++ {
		if err := stats.Put(key(i), []byte("{}"), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := stats.Put([]byte("9999-unreadable"), []byte("{}"), nil); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)*9/10, 10)+"B")

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan string)
	go func() {
		var b strings.Builder
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			b.WriteString(sc.Text() + "\n")
		}
		out <- b.String()
	}()
	stdout := os.Stdout
	os.Stdout = pw
	err = checkAndManageLogSize(host, c)
	os.Stdout = stdout
	pw.Close()
	printed := <-out
	pr.Close()
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(printed, "Error parsing key time") {
		t.Error("retention read statistics newer than its cutoff")
	}
	if ok, _ := stats.Has(key(0), nil); ok {
		t.Error("the oldest statistics entry survived")
	}
	if ok, _ := stats.Has(key(249), nil); !ok {
		t.Error("the newest statistics entry was deleted")
	}
}

func setFreeDisk(t *testing.T, free uint64) {
	saved := util.FreeDiskBytes
	t.Cleanup(func() { util.FreeDiskBytes = saved })
	util.FreeDiskBytes = func() (uint64, error) { return free, nil }
}

type fullDiskStorage struct {
	storage.Storage
	full       *atomic.Bool
	onlyTables bool
}

type fullDiskWriter struct {
	storage.Writer
	full *atomic.Bool
	name string
}

type closingLock struct {
	storage.Locker
	stor storage.Storage
}

func (l closingLock) Unlock() {
	l.Locker.Unlock()
	l.stor.Close()
}

func noSpace(op, name string) error {
	return &os.PathError{Op: op, Path: name, Err: syscall.ENOSPC}
}

func (s fullDiskStorage) Lock() (storage.Locker, error) {
	l, err := s.Storage.Lock()
	if err != nil {
		return nil, err
	}
	return closingLock{l, s.Storage}, nil
}

func (s fullDiskStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	if s.full.Load() && (!s.onlyTables || fd.Type == storage.TypeTable) {
		return nil, noSpace("open", fd.String())
	}
	w, err := s.Storage.Create(fd)
	if err != nil || s.onlyTables {
		return w, err
	}
	return fullDiskWriter{w, s.full, fd.String()}, nil
}

func (s fullDiskStorage) SetMeta(fd storage.FileDesc) error {
	if s.full.Load() && !s.onlyTables {
		return noSpace("write", "CURRENT")
	}
	return s.Storage.SetMeta(fd)
}

func (w fullDiskWriter) Write(p []byte) (int, error) {
	if w.full.Load() {
		return 0, noSpace("write", w.name)
	}
	return w.Writer.Write(p)
}

func TestRetentionEndsItsRunOnAFullDiskAndRecoversOnceSpaceReturns(t *testing.T) {
	holdCleanup(t)
	host, c := "FullDiskHost", "app"
	full := new(atomic.Bool)
	for _, dbType := range util.PrunableDBs() {
		registerDB(t, host, c, dbType, fullDiskStorage{openStorage(t, host, c, dbType), full, false}, nil)
	}
	seedRetentionRows(t, host, []retentionSeed{{c, 400, 0}})
	limit := prunableBytes(host, c) / 2
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(limit, 10)+"B")
	rounds := countRounds(t, nil)
	n := 0
	put := func() error {
		n++
		ts := retentionBase.Add(time.Hour + time.Duration(n)*time.Second).Format("2006-01-02T15:04:05.000000000Z")
		return PutLogMessage(util.GetDB(host, c, "logs"), host, c, []string{ts, "INFO line"})
	}

	full.Store(true)
	setFreeDisk(t, 0)
	if err := put(); err == nil {
		t.Error("a line was reported stored on a full disk")
	}

	setFreeDisk(t, 1<<40)
	done := make(chan error, 1)
	go func() { done <- checkAndManageLogSize(host, c) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("retention reported success on a full disk")
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("retention was still retrying after %d rounds on a full disk", *rounds)
	}
	if *rounds != 1 {
		t.Errorf("retention ran %d rounds on a full disk, want it to stop after the first failure", *rounds)
	}

	full.Store(false)
	if err := put(); err != nil {
		t.Fatalf("ingestion still fails after space returned: %v", err)
	}
	level := "info"
	if got := GetLogs(true, false, host, c, "", maxLogsPerRequest, "", false, &level)["logs"].([][]string); len(got) != 1 {
		t.Errorf("the severity filter finds %d of the 1 line stored after the outage", len(got))
	}
	if err := checkAndManageLogSize(host, c); err != nil {
		t.Fatalf("retention still fails after space returned: %v", err)
	}
	if size := prunableBytes(host, c); size > limit {
		t.Errorf("cleanup returned with %d bytes over the %d byte limit", size, limit)
	}
}

func TestRetentionDropsTheSeverityOfRowsWhoseCompactionFailed(t *testing.T) {
	holdCleanup(t)
	host, c := "NoTablesHost", "app"
	full := new(atomic.Bool)
	for _, dbType := range util.PrunableDBs() {
		registerDB(t, host, c, dbType, fullDiskStorage{openStorage(t, host, c, dbType), full, true}, nil)
	}
	seedRetentionRows(t, host, []retentionSeed{{c, 300, 0}})
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)/2, 10)+"B")
	before := logKeys(host, c)

	full.Store(true)
	if err := checkAndManageLogSize(host, c); err == nil {
		t.Error("retention reported success although compaction failed")
	}
	full.Store(false)

	after := logKeys(host, c)
	if len(after) == len(before) {
		t.Fatal("the deletes were not written")
	}
	statuses := util.GetDB(host, c, "statuses")
	for _, key := range before[:len(before)-len(after)] {
		if ok, _ := statuses.Has([]byte(key), nil); ok {
			t.Fatalf("deleted row %s kept its severity entry", key)
		}
	}
}

func TestRetentionDeletesNothingWithoutRoomToCompact(t *testing.T) {
	holdCleanup(t)
	host, c := "NoRoomHost", "app"
	seedRetentionRows(t, host, []retentionSeed{{c, 300, 0}})
	t.Setenv("MAX_LOGS_SIZE", strconv.FormatInt(prunableBytes(host, c)/2, 10)+"B")
	setFreeDisk(t, 0)

	before := logKeys(host, c)
	if err := checkAndManageLogSize(host, c); err == nil {
		t.Error("retention reported success without room to compact")
	}
	if after := logKeys(host, c); !slices.Equal(after, before) {
		t.Errorf("retention deleted %d rows it had no room to reclaim", len(before)-len(after))
	}
}

func TestRetentionRetriesAFailedCheckAfterTheCooldownWithoutAnotherLine(t *testing.T) {
	holdCleanup(t)
	host, c := "FailedCheckHost", "idle"
	const limit = 256 * 1024
	t.Setenv("MAX_LOGS_SIZE", strconv.Itoa(limit)+"B")
	full := new(atomic.Bool)
	full.Store(true)
	saved := util.FreeDiskBytes
	t.Cleanup(func() { util.FreeDiskBytes = saved })
	util.FreeDiskBytes = func() (uint64, error) {
		if full.Load() {
			return 0, nil
		}
		return 1 << 40, nil
	}
	logCleanupMu.Lock()
	savedCooldown := cleanupCooldown
	cleanupCooldown = time.Second
	logCleanupMu.Unlock()
	t.Cleanup(func() {
		logCleanupMu.Lock()
		cleanupCooldown = savedCooldown
		logCleanupMu.Unlock()
	})
	seedRetentionRows(t, host, []retentionSeed{{c, 250, 0}})
	idle := func() bool {
		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()
		return !isCleanupRunning && !nextCleanup.IsZero()
	}
	waitFor := func(cond func() bool) bool {
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
			if cond() {
				return true
			}
		}
		return false
	}

	logCleanupMu.Lock()
	cleanupPending, nextCleanup = false, time.Time{}
	logCleanupMu.Unlock()
	MaybeScheduleCleanup(host, c)
	if !waitFor(idle) || len(logKeys(host, c)) != 250 {
		t.Fatal("precondition: the check on the full disk did not end without deleting")
	}

	full.Store(false)
	if !waitFor(func() bool { return idle() && prunableBytes(host, c) <= limit }) {
		t.Fatalf("the idle store is still %d bytes against a %d byte limit after space was freed", prunableBytes(host, c), limit)
	}
}
