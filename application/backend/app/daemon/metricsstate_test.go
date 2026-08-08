package daemon

import (
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

	if !h.finalizeStream("c", 7) {
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
