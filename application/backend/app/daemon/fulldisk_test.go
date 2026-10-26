package daemon

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"

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

func TestReattachAfterAFullDiskStoresEachLineOnce(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MAX_LOGS_SIZE", "10GB")
	host, container := util.GetHost(), "CursorAfterFullDisk"
	full := new(atomic.Bool)
	saved := util.FreeDiskBytes
	t.Cleanup(func() { util.FreeDiskBytes = saved })
	util.FreeDiskBytes = func() (uint64, error) {
		if full.Load() {
			return 0, nil
		}
		return 1 << 40, nil
	}
	stor, err := storage.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/streamstate", false)
	if err != nil {
		t.Fatal(err)
	}
	state, err := leveldb.Open(fullDiskStorage{stor, full}, nil)
	if err != nil {
		t.Fatal(err)
	}
	vars.DBMutex.Lock()
	vars.StreamState_DBs[host+"/"+container] = state
	vars.DBMutex.Unlock()
	t.Cleanup(func() {
		for _, kind := range []string{"logs", "statuses", "streamstate"} {
			util.ResetDB(host, container, kind)
		}
	})

	db := util.GetDB(host, container, "logs")
	first := NewDaemonService(nil)
	for i := 0; i < 10; i++ {
		full.Store(i == 1)
		first.ingestLine(host, container, db, "", false, time.Time{}, corpusLine(i))
	}

	second := NewDaemonService(nil)
	cursor := second.storedThrough(host, container)
	second.seedBoundaryFingerprints(host, container, cursor)
	for i := 0; i < 10; i++ {
		second.ingestLine(host, container, db, "", false, cursor, corpusLine(i))
	}
	assertNoDuplicateMessages(t, host, container)
}
