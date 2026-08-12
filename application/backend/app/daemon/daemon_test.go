package daemon

import (
	"context"
	"io"
	"strings"
	"testing"
)

func Test_streamDockerLogsRawTTY(t *testing.T) {
	ctrl := &DaemonService{}
	lines := []string{}
	rc := io.NopCloser(strings.NewReader("2026-01-01T00:00:00.000000000Z hello\n2026-01-01T00:00:01.000000000Z world\n"))

	err := ctrl.streamDockerLogs(context.Background(), rc, func(line string) {
		lines = append(lines, line)
	}, false)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "2026-01-01T00:00:00.000000000Z hello" {
		t.Fatalf("unexpected first line: %s", lines[0])
	}
}
