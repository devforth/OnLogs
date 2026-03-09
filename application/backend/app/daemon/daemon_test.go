package daemon

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
)

func Test_validateMessage(t *testing.T) {
	type args struct {
		message string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{"Bad message", args{message: string([]byte{10})}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := validateMessage(tt.args.message)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateMessage() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("validateMessage() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

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
