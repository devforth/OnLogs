package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func noServices() []string { return nil }

func initInBackground(t *testing.T) (<-chan error, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- SendInitRequest(ctx, noServices) }()
	return done, cancel
}

func waitInit(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("SendInitRequest did not return within %s", within)
		return nil
	}
}

func closedAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
	return l.Addr().String()
}

func TestSendInitRequestWaitsForAnUnreachableMaster(t *testing.T) {
	addr := closedAddr(t)
	t.Setenv("HOST", "http://"+addr)
	done, _ := initInBackground(t)

	time.Sleep(200 * time.Millisecond)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	master := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	master.Listener.Close()
	master.Listener = l
	master.Start()
	t.Cleanup(master.Close)

	if err := waitInit(t, done, 5*time.Second); err != nil {
		t.Fatalf("registration failed once the master was back: %v", err)
	}
}

func TestSendInitRequestRetriesWhileTheMasterFails(t *testing.T) {
	var calls atomic.Int32
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusBadGateway)
		case 2:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(master.Close)
	t.Setenv("HOST", master.URL)
	done, _ := initInBackground(t)

	if err := waitInit(t, done, 10*time.Second); err != nil || calls.Load() != 3 {
		t.Fatalf("got %v after %d calls, want success on the third", err, calls.Load())
	}
}

func TestSendInitRequestGivesUpOnARejectedToken(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		var calls atomic.Int32
		master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(status)
		}))
		t.Setenv("HOST", master.URL)
		done, _ := initInBackground(t)

		if err := waitInit(t, done, 2*time.Second); !errors.Is(err, ErrTokenRejected) || calls.Load() != 1 {
			t.Errorf("%d: got %v after %d calls, want a token error after one", status, err, calls.Load())
		}
		master.Close()
	}
}

func TestSendInitRequestStopsWithItsContext(t *testing.T) {
	t.Setenv("HOST", "http://"+closedAddr(t))
	done, cancel := initInBackground(t)
	time.Sleep(50 * time.Millisecond)
	cancel()

	if err := waitInit(t, done, 500*time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestRegistrationWarningHidesTheHostPassword(t *testing.T) {
	var calls atomic.Int32
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	t.Cleanup(master.Close)
	t.Setenv("HOST", strings.Replace(master.URL, "http://", "http://agent:s3cret-pw@", 1))

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = SendInitRequest(context.Background(), noServices)
	os.Stdout = stdout
	w.Close()
	printed, _ := io.ReadAll(r)
	r.Close()
	if out := string(printed); err != nil || !strings.Contains(out, "502") || strings.Contains(out, "s3cret-pw") {
		t.Fatalf("got %v, want a 502 warning without the password: %q", err, out)
	}
}

func TestRegistrationSendsTheServicesRunningAtThatAttempt(t *testing.T) {
	var running atomic.Value
	running.Store([]string{"w0"})
	sent := make(chan []string, 2)
	var calls atomic.Int32
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Services []string }
		json.NewDecoder(r.Body).Decode(&body)
		sent <- body.Services
		if calls.Add(1) == 1 {
			running.Store([]string{"w1"})
			w.WriteHeader(http.StatusBadGateway)
		}
	}))
	t.Cleanup(master.Close)
	t.Setenv("HOST", master.URL)

	if err := SendInitRequest(context.Background(), func() []string { return running.Load().([]string) }); err != nil {
		t.Fatal(err)
	}
	<-sent
	if got := <-sent; !slices.Equal(got, []string{"w1"}) {
		t.Fatalf("registered with %q, want the services running then, [w1]", got)
	}
}
