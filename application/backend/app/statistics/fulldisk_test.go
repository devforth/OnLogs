package statistics

import (
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

type fullDiskStorage struct {
	storage.Storage
	full *atomic.Bool
}

type fullDiskWriter struct {
	storage.Writer
	full *atomic.Bool
}

type closingLock struct {
	storage.Locker
	stor storage.Storage
}

func (l closingLock) Unlock() {
	l.Locker.Unlock()
	l.stor.Close()
}

func (s fullDiskStorage) Lock() (storage.Locker, error) {
	l, err := s.Storage.Lock()
	if err != nil {
		return nil, err
	}
	return closingLock{l, s.Storage}, nil
}

func (s fullDiskStorage) Create(fd storage.FileDesc) (storage.Writer, error) {
	w, err := s.Storage.Create(fd)
	if err != nil {
		return nil, err
	}
	return fullDiskWriter{w, s.full}, nil
}

func (w fullDiskWriter) Write(p []byte) (int, error) {
	if w.full.Load() {
		return 0, syscall.ENOSPC
	}
	return w.Writer.Write(p)
}

func TestStatsAreSavedAgainOnceSpaceIsFreed(t *testing.T) {
	host, container := "StatsFullDisk", "svc"
	os.RemoveAll("leveldb/hosts/" + host)
	full := new(atomic.Bool)
	saved := util.FreeDiskBytes
	util.FreeDiskBytes = func() (uint64, error) {
		if full.Load() {
			return 0, nil
		}
		return 1 << 40, nil
	}
	t.Cleanup(func() {
		util.FreeDiskBytes = saved
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, container, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, host+"/"+container)
		vars.Mutex.Unlock()
	})
	stor, err := storage.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statistics", false)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := leveldb.Open(fullDiskStorage{stor, full}, nil)
	if err != nil {
		t.Fatal(err)
	}
	vars.DBMutex.Lock()
	vars.Stat_Containers_DBs[host+"/"+container] = stats
	vars.DBMutex.Unlock()

	stamp := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < 3; i++ {
		line := []string{stamp.Add(time.Duration(i) * time.Second).Format(fixedWidthNanos), "ERROR boom"}
		if err := containerdb.PutLogMessage(util.GetDB(host, container, "logs"), host, container, line); err != nil {
			t.Fatal(err)
		}
		full.Store(i == 1)
		restartStats(host, container)
	}

	newest := stamp.Add(2 * time.Second).Format(fixedWidthNanos)
	if cursor := getLastStatTime(util.GetDB(host, container, "statistics")); cursor != newest {
		t.Errorf("the statistics cursor stayed at %s, not %s: every flush rescans the rows since", cursor, newest)
	}
	if got := GetStatisticsByService(host, container, 48)["error"]; got != 3 {
		t.Errorf("3 rows stored, statistics count %d", got)
	}
}
