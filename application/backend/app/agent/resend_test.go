package agent

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/util"
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
	value, err := buffer.Get([]byte("2026-02-10T12:56:09.230421754Z"), nil)
	if err != nil {
		t.Fatalf("the undelivered line was not buffered: %v", err)
	}
	if string(value) != "a line" {
		t.Fatalf("buffered the wrong value: %q", value)
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
