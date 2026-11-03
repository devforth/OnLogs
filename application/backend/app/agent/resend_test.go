package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
)

func TestSendLogMessageBuffersInsteadOfPanicking(t *testing.T) {
	t.Chdir(t.TempDir())
	os.Setenv("HOST", "http://127.0.0.1:1")

	container := "bufferedcontainer"
	panicked := make(chan interface{}, 1)
	go func() {
		defer func() { panicked <- recover() }()
		SendLogMessage("token", container, []string{"2026-02-10T12:56:09.230421754Z", "a line"})
	}()
	if r := <-panicked; r != nil {
		t.Fatalf("a failed delivery panicked instead of buffering: %v", r)
	}

	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	if buffer == nil {
		t.Fatal("no broken-logs buffer was opened")
	}
	iter := buffer.NewIterator(leveldbUtil.BytesPrefix([]byte("2026-02-10T12:56:09.230421754Z +")), nil)
	defer iter.Release()
	if !iter.Next() {
		t.Fatal("the undelivered line was not buffered")
	}
	if string(iter.Value()) != "a line" {
		t.Fatalf("buffered the wrong value: %q", iter.Value())
	}
}

func TestTryResendDeliversAndDrainsTheBuffer(t *testing.T) {
	t.Chdir(t.TempDir())
	container := "resendcontainer"

	if err := os.MkdirAll("leveldb/hosts/"+util.GetHost()+"/containers/"+container, 0o700); err != nil {
		t.Fatal(err)
	}

	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	if buffer == nil {
		t.Fatal("could not open the broken-logs buffer")
	}
	for _, ts := range []string{"2026-02-10T12:56:09.000000001Z", "2026-02-10T12:56:09.000000002Z"} {
		if err := buffer.Put([]byte(ts), []byte("buffered "+ts), nil); err != nil {
			t.Fatal(err)
		}
	}

	delivered := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delivered++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	os.Setenv("HOST", server.URL)

	TryResend()

	if delivered != 2 {
		t.Fatalf("expected both buffered lines to be resent, got %d", delivered)
	}
	iter := buffer.NewIterator(nil, nil)
	defer iter.Release()
	if iter.Next() {
		t.Fatalf("the buffer still holds %q after a successful resend", iter.Key())
	}
}

func queueTwoRecords(t *testing.T, containers ...string) {
	t.Helper()
	for _, container := range containers {
		t.Cleanup(func() { util.ResetDB(util.GetHost(), container, "brokenlogs") })
		buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
		if buffer == nil {
			t.Fatal("could not open the broken-logs buffer")
		}
		for _, ts := range []string{"2026-02-10T12:56:09.000000001Z", "2026-02-10T12:56:09.000000002Z"} {
			if err := buffer.Put([]byte(ts), []byte(container), nil); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func queuedCount(t *testing.T, container string) int {
	t.Helper()
	iter := util.GetDB(util.GetHost(), container, "brokenlogs").NewIterator(nil, nil)
	defer iter.Release()
	n := 0
	for iter.Next() {
		n++
	}
	return n
}

func TestResendFailureInOneQueueDoesNotStarveLaterQueues(t *testing.T) {
	t.Chdir(t.TempDir())
	queueTwoRecords(t, "a-dropped", "b-rejected", "z-healthy")

	var mu sync.Mutex
	attempts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var item struct{ Container string }
		_ = json.NewDecoder(r.Body).Decode(&item)
		mu.Lock()
		attempts[item.Container]++
		mu.Unlock()
		switch item.Container {
		case "a-dropped":
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				panic(err)
			}
			_ = conn.Close()
		case "b-rejected":
			// A failure worth retrying; a 400 is dropped instead.
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("HOST", server.URL)

	for pass := 1; pass <= 2; pass++ {
		TryResend()
		mu.Lock()
		got := map[string]int{"a-dropped": attempts["a-dropped"], "b-rejected": attempts["b-rejected"], "z-healthy": attempts["z-healthy"]}
		mu.Unlock()
		want := map[string]int{"a-dropped": pass, "b-rejected": pass, "z-healthy": 2}
		for container, n := range want {
			if got[container] != n {
				t.Errorf("pass %d: %s attempted %d times, want %d", pass, container, got[container], n)
			}
		}
		if n := queuedCount(t, "z-healthy"); n != 0 {
			t.Errorf("pass %d: z-healthy still holds %d records", pass, n)
		}
		for _, container := range []string{"a-dropped", "b-rejected"} {
			if n := queuedCount(t, container); n != 2 {
				t.Errorf("pass %d: %s holds %d records, want 2", pass, container, n)
			}
		}
	}
}

func TestResendStopsThePassWhenTheMasterCannotBeDialled(t *testing.T) {
	t.Chdir(t.TempDir())
	queueTwoRecords(t, "a-unreachable", "z-unreachable")

	var dials atomic.Int32
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials.Add(1)
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	client.Transport = transport
	t.Cleanup(func() { client.Transport = nil })
	t.Setenv("HOST", "http://127.0.0.1:1")

	TryResend()

	if n := dials.Load(); n != 1 {
		t.Fatalf("expected the pass to stop after one failed dial, got %d", n)
	}
	for _, container := range []string{"a-unreachable", "z-unreachable"} {
		if n := queuedCount(t, container); n != 2 {
			t.Errorf("%s holds %d records, want 2", container, n)
		}
	}
}

func silentMaster(t *testing.T, held string) func(path string) int {
	release := make(chan struct{})
	var mu sync.Mutex
	answered := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == held {
			<-release
			return
		}
		mu.Lock()
		answered[r.URL.Path]++
		mu.Unlock()
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	t.Setenv("HOST", server.URL)
	if client.Timeout <= 0 {
		t.Fatal("requests to the master have no deadline")
	}
	timeout := client.Timeout
	client.Timeout = 500 * time.Millisecond
	t.Cleanup(func() { client.Timeout = timeout })
	return func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return answered[path]
	}
}

func finishes(t *testing.T, what string, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		call()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s is still waiting for a response that never comes", what)
	}
}

func TestSendLogMessageBuffersWhenTheMasterNeverAnswers(t *testing.T) {
	t.Chdir(t.TempDir())
	silentMaster(t, "/api/v1/addLogLine")
	container := "silentcontainer"
	t.Cleanup(func() { util.ResetDB(util.GetHost(), container, "brokenlogs") })

	delivered := true
	finishes(t, "SendLogMessage", func() {
		delivered, _ = SendLogMessage("token", container, []string{"2026-02-10T12:56:09.230421754Z", "a line"})
	})
	if delivered {
		t.Fatal("an unanswered line was reported as delivered")
	}
	if n := queuedCount(t, container); n != 1 {
		t.Fatalf("expected the unanswered line to be queued, got %d records", n)
	}
}

func TestUnansweredHostUpdateDoesNotStopTheDeletePollOrResend(t *testing.T) {
	t.Chdir(t.TempDir())
	answered := silentMaster(t, "/api/v1/addHost")
	queueTwoRecords(t, "pending")

	finishes(t, "the maintenance tick", func() {
		SendUpdate([]string{"pending"})
		TryResend()
	})
	if n := answered("/api/v1/askForDelete"); n != 1 {
		t.Errorf("askForDelete was polled %d times, want 1", n)
	}
	if n := queuedCount(t, "pending"); n != 0 {
		t.Errorf("pending still holds %d records after the resend", n)
	}
}

// Live viewers get lines in arrival order, so the queue has to go first.
func TestSendLogMessageDeliversQueuedLinesFirst(t *testing.T) {
	t.Chdir(t.TempDir())
	container := "orderedcontainer"
	t.Cleanup(func() { util.ResetDB(util.GetHost(), container, "brokenlogs") })
	t.Setenv("HOST", "http://127.0.0.1:1")
	SendLogMessage("token", container, []string{"2026-02-10T12:56:09.000000001Z", "queued"})

	var mu sync.Mutex
	var got []string
	reject := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var item struct{ LogLine []string }
		_ = json.NewDecoder(r.Body).Decode(&item)
		mu.Lock()
		defer mu.Unlock()
		if reject {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		got = append(got, item.LogLine[1])
	}))
	defer server.Close()
	t.Setenv("HOST", server.URL)

	SendLogMessage("token", container, []string{"2026-02-10T12:56:10.000000001Z", "behind"})
	if n := queuedCount(t, container); n != 2 {
		t.Fatalf("with the queue stuck, %d lines are queued, want 2", n)
	}

	mu.Lock()
	reject = false
	mu.Unlock()
	SendLogMessage("token", container, []string{"2026-02-10T12:56:11.000000001Z", "live"})
	if want := []string{"queued", "behind", "live"}; !slices.Equal(got, want) {
		t.Errorf("the master got %v, want %v", got, want)
	}
	if n := queuedCount(t, container); n != 0 {
		t.Errorf("%d lines are still queued", n)
	}
}

// A line the master refuses for good must not hold back the lines after it.
func TestALineTheMasterRejectsDoesNotBlockLaterOnes(t *testing.T) {
	t.Chdir(t.TempDir())
	container := "poisoncontainer"
	t.Cleanup(func() { util.ResetDB(util.GetHost(), container, "brokenlogs") })
	var mu sync.Mutex
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var item struct{ LogLine []string }
		_ = json.NewDecoder(r.Body).Decode(&item)
		if item.LogLine[1] == "too big" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		got = append(got, item.LogLine[1])
		mu.Unlock()
	}))
	defer server.Close()

	// Queued while the master was away, then rejected once it is back.
	t.Setenv("HOST", "http://127.0.0.1:1")
	SendLogMessage("token", container, []string{"2026-02-10T12:56:09.000000001Z", "too big"})
	t.Setenv("HOST", server.URL)
	for i, line := range []string{"after-1", "after-2"} {
		SendLogMessage("token", container, []string{fmt.Sprintf("2026-02-10T12:56:1%d.000000001Z", i), line})
	}
	SendLogMessage("token", container, []string{"2026-02-10T12:56:20.000000001Z", "too big"})
	SendLogMessage("token", container, []string{"2026-02-10T12:56:21.000000001Z", "after-3"})

	if want := []string{"after-1", "after-2", "after-3"}; !slices.Equal(got, want) {
		t.Errorf("the master got %v, want %v", got, want)
	}
	if n := queuedCount(t, container); n != 0 {
		t.Errorf("%d rejected lines are still queued", n)
	}
}

// A drained queue must not make every later line slower.
func TestADrainedQueueCostsNothingPerLine(t *testing.T) {
	t.Chdir(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	t.Setenv("HOST", server.URL)
	drained, fresh := "drainedcontainer", "freshcontainer"
	t.Cleanup(func() {
		util.ResetDB(util.GetHost(), drained, "brokenlogs")
		util.ResetDB(util.GetHost(), fresh, "brokenlogs")
	})
	buffer := util.GetDB(util.GetHost(), drained, "brokenlogs")
	for i := range 20000 {
		if err := buffer.Put([]byte(fmt.Sprintf("2026-02-10T12:56:09.%09dZ +ID%d", i, i)), []byte("old"), nil); err != nil {
			t.Fatal(err)
		}
	}
	SendLogMessage("token", drained, []string{"2026-02-10T13:00:00.000000001Z", "drains"})

	timeLines := func(container string) time.Duration {
		started := time.Now()
		for i := range 300 {
			SendLogMessage("token", container, []string{fmt.Sprintf("2026-02-10T14:00:%02d.%09dZ", i/60, i), "line"})
		}
		return time.Since(started)
	}
	base := timeLines(fresh)
	after := timeLines(drained)
	if after > 3*base+50*time.Millisecond {
		t.Errorf("300 lines took %v after a drained queue, %v without one", after, base)
	}
}

// Old agents queued lines without an event id, so a double send is stored twice.
func TestAQueueIsDrainedOnceWhenTwoSendersRace(t *testing.T) {
	t.Chdir(t.TempDir())
	container := "racedcontainer"
	t.Cleanup(func() { util.ResetDB(util.GetHost(), container, "brokenlogs") })
	if err := os.MkdirAll("leveldb/hosts/"+util.GetHost()+"/containers/"+container, 0o700); err != nil {
		t.Fatal(err)
	}
	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	for i := range 100 {
		if err := buffer.Put([]byte(fmt.Sprintf("2026-02-10T12:56:09.%09dZ", i)), []byte(fmt.Sprintf("old-%d", i)), nil); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var item struct{ LogLine []string }
		_ = json.NewDecoder(r.Body).Decode(&item)
		time.Sleep(time.Millisecond)
		mu.Lock()
		seen[item.LogLine[1]]++
		mu.Unlock()
	}))
	defer server.Close()
	t.Setenv("HOST", server.URL)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); TryResend() }()
	go func() {
		defer wg.Done()
		SendLogMessage("token", container, []string{"2026-02-10T13:00:00.000000001Z", "new"})
	}()
	wg.Wait()
	for line, n := range seen {
		if n != 1 {
			t.Errorf("%q was sent %d times", line, n)
		}
	}
	if len(seen) != 101 {
		t.Errorf("the master got %d distinct lines, want 101", len(seen))
	}
}
