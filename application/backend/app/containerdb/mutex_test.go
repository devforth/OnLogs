package containerdb

import (
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func mutexIsFree(d time.Duration) bool {
	acquired := make(chan struct{})
	go func() {
		vars.Mutex.Lock()
		vars.Mutex.Unlock()
		close(acquired)
	}()
	select {
	case <-acquired:
		return true
	case <-time.After(d):
		return false
	}
}

func TestPutLogMessageHandlesAContainerWithNoStatCounterYet(t *testing.T) {
	host := "FirstLineHost"
	container := "FirstLineContainer"
	location := host + "/" + container
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)

	// The first log line from a host/container the process has not seen: no
	// counter has been created for it yet.
	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, location)
	vars.Mutex.Unlock()

	db, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	panicked := make(chan interface{}, 1)
	go func() {
		defer func() { panicked <- recover() }()
		PutLogMessage(db, host, container, []string{vars.Year + "-02-10T12:56:09.230421754Z", "first line"})
	}()

	if r := <-panicked; r != nil {
		t.Errorf("the first log line for a new container panicked: %v", r)
	}

	if !mutexIsFree(3 * time.Second) {
		t.Fatal("vars.Mutex was left locked, which freezes ingestion from every source host-wide")
	}

	vars.Mutex.Lock()
	counter := vars.Container_Stat_Counter[location]
	vars.Mutex.Unlock()
	if counter == nil {
		t.Fatal("no stat counter was created for the container")
	}
	if counter["other"] != 1 {
		t.Errorf("expected the line to be counted once, got %v", counter)
	}
}

func TestPutLogMessageCountsConcurrentlyWithoutRacing(t *testing.T) {
	host := "ConcurrentHost"
	container := "ConcurrentContainer"
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)

	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, host+"/"+container)
	vars.Mutex.Unlock()

	db, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	const writers = 8
	done := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func() {
			var failure error
			defer func() {
				if r := recover(); r != nil {
					failure = errFromPanic(r)
				}
				done <- failure
			}()
			for n := 0; n < 25; n++ {
				PutLogMessage(db, host, container, []string{vars.Year + "-02-10T12:56:09.230421754Z", "concurrent line"})
			}
		}()
	}
	for i := 0; i < writers; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent ingestion panicked: %v", err)
		}
	}

	if !mutexIsFree(3 * time.Second) {
		t.Fatal("vars.Mutex was left locked after concurrent ingestion")
	}
}

type panicError struct{ value interface{} }

func (e panicError) Error() string { return "panic: " + toString(e.value) }

func errFromPanic(r interface{}) error { return panicError{r} }

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	if e, ok := v.(error); ok {
		return e.Error()
	}
	return "unknown"
}
