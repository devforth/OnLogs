package agent

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

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
			w.WriteHeader(http.StatusBadRequest)
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
	http.DefaultClient.Transport = transport
	t.Cleanup(func() { http.DefaultClient.Transport = nil })
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
