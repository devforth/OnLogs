package daemon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/client"
)

func newDrainService(t *testing.T, name string, logs http.HandlerFunc) *DaemonService {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("AGENT", "false")
	t.Setenv("MAX_LOGS_SIZE", "10GB")
	srv := httptest.NewServer(logs)
	// A cancel racing a kept-alive response's end can strand the body read.
	srv.Config.SetKeepAlivesEnabled(false)
	cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.45"))
	if err != nil {
		t.Fatal(err)
	}
	d := NewDaemonService(&docker.DockerService{Client: cli})
	t.Cleanup(func() {
		waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })
		_ = cli.Close()
		srv.Close()
		for _, kind := range []string{"logs", "streamstate", "statistics", "statuses"} {
			util.ResetDB(util.GetHost(), name, kind)
		}
	})
	return d
}

func streamRegistered(d *DaemonService, name string) bool {
	d.streamsMu.Lock()
	defer d.streamsMu.Unlock()
	_, ok := d.streamIDs[name]
	return ok
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func storedMessages(name string) map[string]int {
	counts := map[string]int{}
	iter := util.GetDB(util.GetHost(), name, "logs").NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		counts[string(iter.Value())]++
	}
	return counts
}

func writeRecords(w http.ResponseWriter, recs ...logRecord) {
	for _, rec := range recs {
		_, _ = w.Write(partialFrames(rec))
	}
	w.(http.Flusher).Flush()
}

func records(base time.Time, from int, prefix string, n int) []logRecord {
	out := make([]logRecord, n)
	for i := range out {
		out[i] = logRecord{base.Add(time.Duration(from+i) * time.Millisecond), fmt.Sprintf("%s-%03d", prefix, i)}
	}
	return out
}

func TestStopStreamKeepsLinesDockerStillDelivers(t *testing.T) {
	name := "drain-tail"
	base := time.Now().Add(-10 * time.Second)
	tail := records(base, 1, "tail", 100)
	attached, release := make(chan struct{}), make(chan struct{})
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		writeRecords(w, logRecord{base, "first"})
		close(attached)
		<-release
		writeRecords(w, tail...)
	})

	d.EnsureStream(context.Background(), name)
	<-attached
	d.StopStream(name)
	close(release)
	waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })

	counts := storedMessages(name)
	missing := 0
	for _, rec := range append([]logRecord{{base, "first"}}, tail...) {
		if counts[rec.text] != 1 {
			missing++
		}
	}
	if missing > 0 || counts["ONLOGS: Container listening stopped! (EOF)"] != 1 {
		t.Fatalf("%d of %d lines Docker delivered were not stored once, stopped rows %d",
			missing, len(tail)+1, counts["ONLOGS: Container listening stopped! (EOF)"])
	}
}

func TestStartWhileDrainingAttachesTheNewRun(t *testing.T) {
	name := "drain-restart"
	base := time.Now().Add(-10 * time.Second)
	oldRun := append([]logRecord{{base, "old-first"}}, records(base, 1, "old", 20)...)
	newRun := records(base, 1000, "new", 20)
	attached, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		if requests.Add(1) == 1 {
			writeRecords(w, oldRun[0])
			close(attached)
			<-release
			writeRecords(w, oldRun[1:]...)
			return
		}
		writeRecords(w, append(append([]logRecord{}, oldRun...), newRun...)...)
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	d.EnsureStream(ctx, name)
	<-attached
	d.StopStream(name)
	d.EnsureStream(ctx, name)
	if !slices.Contains(vars.ActiveStreams(), name) {
		t.Error("a start during the drain is not reported as an active stream")
	}
	close(release)

	waitUntil(t, "new run stored", func() bool { return storedMessages(name)[newRun[len(newRun)-1].text] == 1 })
	counts := storedMessages(name)
	for _, rec := range append(append([]logRecord{}, oldRun...), newRun...) {
		if counts[rec.text] != 1 {
			t.Errorf("%q stored %d times", rec.text, counts[rec.text])
		}
	}
}

func TestRunThatStartsAndExitsDuringADrainIsStored(t *testing.T) {
	name := "drain-short-run"
	base := time.Now().Add(-10 * time.Second)
	oldRun := append([]logRecord{{base, "old-first"}}, records(base, 1, "old", 20)...)
	newRun := records(base, 1000, "new", 20)
	attached, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		if requests.Add(1) == 1 {
			writeRecords(w, oldRun[0])
			close(attached)
			<-release
			writeRecords(w, oldRun[1:]...)
			return
		}
		writeRecords(w, append(append([]logRecord{}, oldRun...), newRun...)...)
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	d.EnsureStream(ctx, name)
	<-attached
	d.StopStream(name)
	d.EnsureStream(ctx, name)
	d.StopStream(name)
	close(release)

	waitUntil(t, "the run that started and exited during the drain to be read", func() bool {
		return requests.Load() == 2 && !streamRegistered(d, name)
	})
	counts := storedMessages(name)
	for _, rec := range append(append([]logRecord{}, oldRun...), newRun...) {
		if counts[rec.text] != 1 {
			t.Errorf("%q stored %d times", rec.text, counts[rec.text])
		}
	}
}

func TestDrainingStreamIsReleasedWhenDockerNeverEndsIt(t *testing.T) {
	defer func(d time.Duration) { drainTimeout = d }(drainTimeout)
	drainTimeout = 20 * time.Millisecond

	name := "drain-stuck"
	attached, gone := make(chan struct{}), make(chan struct{})
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		writeRecords(w, logRecord{time.Now().Add(-time.Second), "first"})
		close(attached)
		<-r.Context().Done()
		close(gone)
	})

	d.EnsureStream(context.Background(), name)
	<-attached
	d.StopStream(name)
	select {
	case <-gone:
	case <-time.After(5 * time.Second):
		t.Fatal("a stream Docker never ends was not released after the drain timeout")
	}
	waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })
	if got := storedMessages(name)["ONLOGS: Container listening stopped! (drain timeout)"]; got != 1 {
		t.Errorf("a stream cut by the drain timeout logged %d stopped rows, want 1", got)
	}
}

func TestShutdownDuringDrainWritesNoStoppedRow(t *testing.T) {
	name := "drain-shutdown"
	attached, gone := make(chan struct{}), make(chan struct{})
	d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		writeRecords(w, logRecord{time.Now().Add(-time.Second), "first"})
		close(attached)
		<-r.Context().Done()
		close(gone)
	})
	ctx, cancel := context.WithCancel(context.Background())

	d.EnsureStream(ctx, name)
	<-attached
	d.StopStream(name)
	cancel()
	<-gone
	waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })
	for msg := range storedMessages(name) {
		if strings.HasPrefix(msg, "ONLOGS: Container listening stopped!") {
			t.Errorf("a stream cut by shutdown logged %q", msg)
		}
	}
}
