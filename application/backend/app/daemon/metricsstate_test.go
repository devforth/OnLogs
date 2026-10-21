package daemon

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
)

// finalizeStream clears droppedReplays; the /metrics total must not follow it
// down, or every stream restart looks like a counter reset.
func TestDroppedReplayTotalsSurviveFinalize(t *testing.T) {
	h := NewDaemonService(nil)

	h.streamsMu.Lock()
	h.streamIDs["c"] = 7
	h.streamsMu.Unlock()

	for i := 0; i < 3; i++ {
		h.countDroppedReplay("c")
	}

	if got := h.DroppedReplayTotals()[util.GetHost()+"/c"]; got != 3 {
		t.Fatalf("before finalize: got %d, want 3", got)
	}

	if owned, _ := h.finalizeStream("c", 7); !owned {
		t.Fatal("finalizeStream did not claim the stream")
	}

	if got := h.DroppedReplays("c"); got != 0 {
		t.Errorf("per-stream counter should reset: got %d, want 0", got)
	}
	if got := h.DroppedReplayTotals()[util.GetHost()+"/c"]; got != 3 {
		t.Errorf("monotonic total went backwards across finalize: got %d, want 3", got)
	}

	h.countDroppedReplay("c")
	if got := h.DroppedReplayTotals()[util.GetHost()+"/c"]; got != 4 {
		t.Errorf("after restart: got %d, want 4", got)
	}
}

func TestCursorTimestampsRecorded(t *testing.T) {
	h := NewDaemonService(nil)

	if len(h.CursorTimestamps()) != 0 {
		t.Fatal("expected no cursors before ingestion")
	}

	when := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	h.saveCursor("host-a", "cont-a", when)

	got, ok := h.CursorTimestamps()["host-a/cont-a"]
	if !ok {
		t.Fatal("cursor not recorded")
	}
	if !got.Equal(when) {
		t.Errorf("cursor: got %s, want %s", got, when)
	}

	later := when.Add(time.Minute)
	h.saveCursor("host-a", "cont-a", later)
	if got := h.CursorTimestamps()["host-a/cont-a"]; !got.Equal(later) {
		t.Errorf("cursor not advanced: got %s, want %s", got, later)
	}
}

func TestCursorTimestampsDroppedOnFinalize(t *testing.T) {
	h := NewDaemonService(nil)

	h.streamsMu.Lock()
	h.streamIDs["gone"] = 1
	h.streamsMu.Unlock()

	h.saveCursor(util.GetHost(), "gone", time.Now().UTC())
	if _, ok := h.CursorTimestamps()[util.GetHost()+"/gone"]; !ok {
		t.Fatal("cursor not recorded")
	}

	h.finalizeStream("gone", 1)

	// Otherwise a deliberately stopped container alerts forever.
	if _, ok := h.CursorTimestamps()[util.GetHost()+"/gone"]; ok {
		t.Error("cursor kept after the stream ended")
	}
}

func TestStopStreamClearsTheStoppedStreamsState(t *testing.T) {
	for name, hold := range map[string]bool{"stop-cleanup-eof": false, "stop-cleanup-held": true} {
		t.Run(name, func(t *testing.T) {
			if hold {
				defer func(d time.Duration) { drainTimeout = d }(drainTimeout)
				drainTimeout = 20 * time.Millisecond
			}
			line := logRecord{time.Now().Add(-time.Second), "before stop"}
			release := make(chan struct{})
			d := newDrainService(t, name, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
				writeRecords(w, line, line)
				if hold {
					<-r.Context().Done()
					return
				}
				<-release
			})

			d.EnsureStream(context.Background(), name)
			waitUntil(t, "line stored and replay dropped", func() bool {
				_, cursor := d.CursorTimestamps()[util.GetHost()+"/"+name]
				return cursor && d.DroppedReplays(name) == 1
			})
			d.StopStream(name)
			close(release)
			waitUntil(t, "stream released", func() bool { return !streamRegistered(d, name) })

			d.streamsMu.Lock()
			_, cursor := d.cursorTimestamps[util.GetHost()+"/"+name]
			_, set := d.recentSet[name]
			_, fingerprints := d.recentFingerprints[name]
			_, dropped := d.droppedReplays[name]
			total := d.droppedReplayTotals[util.GetHost()+"/"+name]
			d.streamsMu.Unlock()
			if cursor || set || fingerprints || dropped {
				t.Errorf("stopped stream kept cursor=%t recentSet=%t fingerprints=%t droppedReplays=%t",
					cursor, set, fingerprints, dropped)
			}
			if total != 1 {
				t.Errorf("dropped replay total %d, want 1", total)
			}
		})
	}
}
