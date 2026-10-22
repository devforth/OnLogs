package daemon

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/util"
	"github.com/docker/docker/client"
)

type logRecord struct {
	ts   time.Time
	text string
}

func partialFrames(rec logRecord) []byte {
	const partial = 16 * 1024
	ts := rec.ts.UTC().Format(streamTimestampFmt)
	var out []byte
	for start := 0; ; start += partial {
		end := min(start+partial, len(rec.text))
		payload := ts + " " + rec.text[start:end]
		if end == len(rec.text) {
			payload += "\n"
		}
		header := make([]byte, 8)
		header[0] = 1
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		out = append(append(out, header...), payload...)
		if end == len(rec.text) {
			return out
		}
	}
}

func TestStreamDockerLogsTruncatesAnOversizedLineAndKeepsReading(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var stream []byte
	for _, rec := range []logRecord{{ts, "before"}, {ts.Add(time.Second), strings.Repeat("x", 2*1024*1024)}, {ts.Add(2 * time.Second), "after"}} {
		stream = append(stream, partialFrames(rec)...)
	}

	var lines []string
	err := (&DaemonService{}).streamDockerLogs(context.Background(), io.NopCloser(strings.NewReader(string(stream))), func(line string) error {
		lines = append(lines, line)
		return nil
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 3 || !strings.HasSuffix(lines[0], " before") || !strings.HasSuffix(lines[2], " after") {
		t.Fatalf("expected before, the long line and after, got %d lines", len(lines))
	}
	if !strings.HasSuffix(lines[1], truncatedLineMark) || len(lines[1]) > keptLongLine+len(truncatedLineMark) {
		t.Fatalf("long line not truncated with a mark: %d bytes", len(lines[1]))
	}
	if _, _, ok := parseDockerLogLine(lines[1]); !ok {
		t.Fatal("truncated line lost its timestamp")
	}
}

func TestOversizedLineDoesNotBlockLaterLogsAcrossReattaches(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AGENT", "false")
	t.Setenv("MAX_LOGS_SIZE", "10GB")
	host, name := util.GetHost(), "oversized-line"

	now := time.Now()
	backlog := []logRecord{
		{now.Add(-3 * time.Second), "before"},
		{now.Add(-2 * time.Second), strings.Repeat("x", 2*1024*1024)},
		{now.Add(-1 * time.Second), "after"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/logs") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Config":{"Tty":false}}`))
			return
		}
		sec, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
		for _, rec := range backlog {
			if !rec.ts.Before(time.Unix(sec, 0)) {
				_, _ = w.Write(partialFrames(rec))
			}
		}
	}))
	cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.45"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cli.Close()
		srv.Close()
		for _, kind := range []string{"logs", "streamstate", "statistics", "statuses"} {
			util.ResetDB(host, name, kind)
		}
	})

	d := NewDaemonService(&docker.DockerService{Client: cli})
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		d.runContainerStream(ctx, name, false, 0)
		cancel()
	}

	counts := map[string]int{}
	iter := util.GetDB(host, name, "logs").NewIterator(nil, nil)
	for iter.Next() {
		msg := string(iter.Value())
		if strings.HasSuffix(msg, truncatedLineMark) {
			msg = "truncated"
		}
		counts[msg]++
	}
	iter.Release()

	if counts["before"] != 1 || counts["truncated"] != 1 || counts["after"] != 1 {
		t.Fatalf("expected before, the truncated line and after stored once each, got before=%d truncated=%d after=%d",
			counts["before"], counts["truncated"], counts["after"])
	}
	if counts["ONLOGS: Container listening stopped! (EOF)"] != 2 {
		t.Fatalf("expected both attaches to end cleanly, got %v", counts)
	}
	if cursor := d.storedThrough(host, name); !cursor.Equal(backlog[2].ts.UTC()) {
		t.Fatalf("cursor did not move past the long line: %v", cursor)
	}
}
