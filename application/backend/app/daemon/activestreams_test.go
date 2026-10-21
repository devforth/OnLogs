package daemon

import (
	"context"
	"net/http"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
)

func activeMatchesStreams(d *DaemonService, name string) bool {
	d.streamsMu.Lock()
	defer d.streamsMu.Unlock()
	_, registered := d.streamIDs[name]
	_, draining := d.draining[name]
	listed := slices.Contains(vars.ActiveStreams(), name)
	return (!listed || registered) && (!registered || draining || listed)
}

func TestActiveStreamsMatchStreamStateWhileStartsAndStopsRace(t *testing.T) {
	defer func(d time.Duration) { drainTimeout = d }(drainTimeout)
	drainTimeout = time.Millisecond

	name := "active-race"
	quit := make(chan struct{})
	var requests atomic.Int32
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		w.(http.Flusher).Flush()
		if requests.Add(1)%2 == 0 {
			select {
			case <-r.Context().Done():
			case <-quit:
			}
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	for _, op := range []func(){
		func() { d.EnsureStream(ctx, name) },
		func() { d.StopStream(name) },
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-quit:
					return
				default:
					op()
					runtime.Gosched()
				}
			}
		}()
	}

	checks, mismatches := 0, 0
	deadline := time.Now().Add(10 * time.Second)
	for requests.Load() < 300 && time.Now().Before(deadline) {
		checks++
		if !activeMatchesStreams(d, name) {
			mismatches++
		}
		runtime.Gosched()
	}
	close(quit)
	wg.Wait()

	if mismatches > 0 {
		t.Fatalf("%d of %d checks saw the active list disagree with the registered stream", mismatches, checks)
	}
	if requests.Load() < 300 {
		t.Fatalf("only %d streams attached before the deadline", requests.Load())
	}

	d.StopStream(name)
	waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })
	if slices.Contains(vars.ActiveStreams(), name) {
		t.Error("a stopped and released stream is still listed as active")
	}
}

func TestStopAfterAStartDuringDrainKeepsTheStreamStopped(t *testing.T) {
	name := "drain-stop-again"
	attached, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		if requests.Add(1) == 1 {
			writeRecords(w, logRecord{time.Now().Add(-time.Second), "first"})
			close(attached)
			<-release
			return
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	d.EnsureStream(ctx, name)
	<-attached
	d.StopStream(name)
	d.EnsureStream(ctx, name)
	d.StopStream(name)
	if !activeMatchesStreams(d, name) || slices.Contains(vars.ActiveStreams(), name) {
		t.Error("a stream stopped again during its drain is still listed as active or will restart")
	}
	d.streamsMu.Lock()
	first := d.streamIDs[name]
	d.streamsMu.Unlock()
	close(release)

	waitUntil(t, "old run released", func() bool {
		d.streamsMu.Lock()
		defer d.streamsMu.Unlock()
		id, ok := d.streamIDs[name]
		return !ok || id != first
	})
	waitUntil(t, "the run started during the drain to be read to its end", func() bool {
		return requests.Load() == 2 && !streamRegistered(d, name)
	})
	if slices.Contains(vars.ActiveStreams(), name) {
		t.Error("a stream stopped again during its drain is still listed as active")
	}
}
