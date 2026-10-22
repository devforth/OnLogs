package daemon

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

func clearedThenFresh(cleared []logRecord, requests *atomic.Int32, holdFirst func(*http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		writeRecords(w, cleared...)
		if requests.Add(1) == 1 {
			holdFirst(r)
			return
		}
		writeRecords(w, logRecord{time.Now(), "after-clear"})
	}
}

func assertClearStuck(t *testing.T, name string, cleared []logRecord) {
	t.Helper()
	counts := storedMessages(name)
	restored := 0
	for _, rec := range cleared {
		restored += counts[rec.text]
	}
	if restored > 0 || counts["after-clear"] != 1 {
		t.Fatalf("after a clear, %d of %d cleared lines came back and the new line was stored %d times",
			restored, len(cleared), counts["after-clear"])
	}
}

func TestClearedLogsStayClearedWhenTheStreamReattaches(t *testing.T) {
	for _, tc := range []struct {
		order     string
		stopFirst bool
	}{{"eof first", false}, {"stop first", true}} {
		t.Run(tc.order, func(t *testing.T) {
			name := "clear-reattach"
			cleared := records(time.Now().Add(-10*time.Second), 0, "cleared", maxRecentLogEntries+1)
			release := make(chan struct{})
			var requests atomic.Int32
			d := newDrainService(t, name, clearedThenFresh(cleared, &requests, func(*http.Request) { <-release }))

			d.EnsureStream(context.Background(), name)
			waitUntil(t, "cleared lines stored", func() bool { return storedMessages(name)[cleared[len(cleared)-1].text] == 1 })
			containerdb.DeleteContainer(util.GetHost(), name, false)
			if tc.stopFirst {
				d.StopStream(name)
			}
			close(release)
			waitUntil(t, "first stream ended", func() bool { return !streamRegistered(d, name) })

			d.EnsureStream(context.Background(), name)
			waitUntil(t, "second stream ended", func() bool { return requests.Load() == 2 && !streamRegistered(d, name) })
			assertClearStuck(t, name, cleared)
		})
	}
}

func TestClearedLogsStayClearedAfterARestart(t *testing.T) {
	name := "clear-restart"
	cleared := records(time.Now().Add(-10*time.Second), 0, "cleared", 2)
	var requests atomic.Int32
	before := newDrainService(t, name, clearedThenFresh(cleared, &requests, func(r *http.Request) { <-r.Context().Done() }))
	ctx, shutdown := context.WithCancel(context.Background())

	before.EnsureStream(ctx, name)
	waitUntil(t, "cleared lines stored", func() bool { return storedMessages(name)[cleared[1].text] == 1 })
	containerdb.DeleteContainer(util.GetHost(), name, false)
	shutdown()
	waitUntil(t, "first stream ended", func() bool { return !streamRegistered(before, name) })
	for _, kind := range []string{"logs", "streamstate", "statistics", "statuses"} {
		util.ResetDB(util.GetHost(), name, kind)
	}

	after := NewDaemonService(before.DockerClient)
	after.EnsureStream(context.Background(), name)
	waitUntil(t, "second stream ended", func() bool { return requests.Load() == 2 && !streamRegistered(after, name) })
	assertClearStuck(t, name, cleared)
}
