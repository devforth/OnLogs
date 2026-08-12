package daemon

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// A reader that never reaches EOF, so StdCopy keeps producing and eventually
// parks in pw.Write once the consumer stops reading.
type endlessReader struct{ frame []byte }

func (r *endlessReader) Read(p []byte) (int, error) {
	n := copy(p, r.frame)
	return n, nil
}
func (r *endlessReader) Close() error { return nil }

func dockerFrame(payload string) []byte {
	header := []byte{1, 0, 0, 0, 0, 0, 0, byte(len(payload))}
	return append(header, []byte(payload)...)
}

// Cancelling the context must not leave streamDockerLogs waiting on a StdCopy
// that is parked writing into a pipe nobody reads. If it does, the stream never
// finalises and EnsureStream refuses to restart that container ever again.
func TestStreamDockerLogsReturnsWhenCancelledMidWrite(t *testing.T) {
	ctrl := &DaemonService{}
	ctx, cancel := context.WithCancel(context.Background())

	reader := &endlessReader{frame: dockerFrame("2026-01-01T00:00:00.000000000Z x\n")}

	done := make(chan error, 1)
	go func() {
		done <- ctrl.streamDockerLogs(ctx, reader, func(string) {}, true)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("streamDockerLogs never returned after cancellation; the container's stream is wedged for the life of the process")
	}
}

// A single line larger than the scanner's buffer must end the stream rather than
// wedge it, so the reconcile loop can re-attach.
func TestStreamDockerLogsReturnsOnAnOversizedLine(t *testing.T) {
	ctrl := &DaemonService{}

	huge := "2026-01-01T00:00:00.000000000Z " + strings.Repeat("a", 2*1024*1024)
	reader := io.NopCloser(strings.NewReader(string(dockerFrameLarge(huge))))

	done := make(chan error, 1)
	go func() {
		done <- ctrl.streamDockerLogs(context.Background(), reader, func(string) {}, true)
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Log("oversized line was tolerated")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("streamDockerLogs never returned on an oversized line")
	}
}

func dockerFrameLarge(payload string) []byte {
	n := len(payload)
	header := []byte{1, 0, 0, 0, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	return append(header, []byte(payload)...)
}
