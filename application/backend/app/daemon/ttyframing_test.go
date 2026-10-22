package daemon

import (
	"context"
	"fmt"
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

func TestStreamKeepsTTYLogsWhenInspectFails(t *testing.T) {
	for _, tc := range []struct {
		name           string
		tty, inspectOK bool
	}{
		{"tty-inspect-fails", true, false},
		{"tty-inspect-ok", true, true},
		{"mux-inspect-fails", false, false},
		{"mux-inspect-ok", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			t.Setenv("AGENT", "false")
			t.Setenv("MAX_LOGS_SIZE", "10GB")
			host := util.GetHost()

			now := time.Now()
			backlog := []logRecord{{now.Add(-25 * time.Second), "first"}, {now.Add(-time.Second), "second"}}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, "/logs") {
					if !tc.inspectOK {
						http.Error(w, `{"message":"inspect failed"}`, http.StatusInternalServerError)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"Config":{"Tty":%t}}`, tc.tty)
					return
				}
				sec, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
				contentType := "application/vnd.docker.multiplexed-stream"
				if tc.tty {
					contentType = "application/vnd.docker.raw-stream"
				}
				w.Header().Set("Content-Type", contentType)
				for _, rec := range backlog {
					if rec.ts.Before(time.Unix(sec, 0)) {
						continue
					}
					if tc.tty {
						_, _ = w.Write([]byte(rec.ts.UTC().Format(streamTimestampFmt) + " " + rec.text + "\n"))
					} else {
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
					util.ResetDB(host, tc.name, kind)
				}
			})

			d := NewDaemonService(&docker.DockerService{Client: cli})
			for i := 0; i < 2; i++ {
				ctx, cancel := context.WithCancel(context.Background())
				d.runContainerStream(ctx, tc.name, false, 0)
				cancel()
			}

			counts := map[string]int{}
			iter := util.GetDB(host, tc.name, "logs").NewIterator(nil, nil)
			for iter.Next() {
				counts[string(iter.Value())]++
			}
			iter.Release()

			if counts["first"] != 1 || counts["second"] != 1 || counts["ONLOGS: Container listening stopped! (EOF)"] != 2 {
				t.Fatalf("expected each line stored once and both attaches to end cleanly, got %v", counts)
			}
			if cursor := d.storedThrough(host, tc.name); !cursor.Equal(backlog[1].ts.UTC()) {
				t.Fatalf("cursor not at the last line: %v", cursor)
			}
		})
	}
}
