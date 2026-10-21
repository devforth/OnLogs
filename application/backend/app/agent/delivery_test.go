package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/client"
)

const (
	sharedTS = "2026-10-19T12:00:00.000000001Z"
	newerTS  = "2026-10-19T12:00:01.000000001Z"
)

type sentLog struct {
	Token, Host, Container, EventID string
	LogLine                         []string
}

type deliveryFixture struct {
	t                            *testing.T
	container, remoteHost, token string
}

func newDeliveryFixture(t *testing.T) *deliveryFixture {
	t.Helper()
	t.Chdir(t.TempDir())
	t.Setenv("MAX_LOGS_SIZE", "1GB")
	t.Setenv("AGENT", "true")
	f := &deliveryFixture{t: t, container: t.Name(), remoteHost: "delivery-receiver", token: db.CreateOnLogsToken()}
	t.Setenv("ONLOGS_TOKEN", f.token)
	t.Cleanup(func() {
		statistics.StopWorker(f.remoteHost, f.container)
		for _, host := range []string{util.GetHost(), f.remoteHost} {
			for _, kind := range []string{"logs", "statistics", "statuses", "brokenlogs", "streamstate"} {
				util.ResetDB(host, f.container, kind)
			}
		}
		_ = vars.TokensDB.Delete([]byte(f.token), nil)
	})
	return f
}

func (f *deliveryFixture) decode(r *http.Request) sentLog {
	var item sentLog
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		panic(err)
	}
	return item
}

func (f *deliveryFixture) persist(w http.ResponseWriter, item sentLog) {
	item.Host = f.remoteHost
	data, err := json.Marshal(item)
	if err != nil {
		panic(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/addLogLine", bytes.NewReader(data))
	(&routes.RouteController{}).AddLogLine(w, r)
}

func (f *deliveryFixture) serve(handler http.HandlerFunc) {
	s := httptest.NewServer(handler)
	f.t.Cleanup(s.Close)
	f.t.Setenv("HOST", s.URL)
}

func (f *deliveryFixture) messages(kind, host string) []string {
	f.t.Helper()
	d := util.GetDB(host, f.container, kind)
	if d == nil {
		f.t.Fatalf("could not open %s/%s", host, kind)
	}
	it := d.NewIterator(nil, nil)
	defer it.Release()
	var result []string
	for it.Next() {
		result = append(result, string(it.Value()))
	}
	if err := it.Error(); err != nil {
		f.t.Fatal(err)
	}
	return result
}

func (f *deliveryFixture) stream(lines ...string) {
	f.t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/logs"):
			w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
			for _, line := range lines {
				fmt.Fprintln(w, line)
			}
		case strings.HasSuffix(r.URL.Path, "/json"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"Config":{"Tty":true}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	cli, err := client.NewClientWithOpts(client.WithHost(s.URL), client.WithVersion("1.45"))
	if err != nil {
		f.t.Fatal(err)
	}
	defer cli.Close()
	d := daemon.NewDaemonService(&docker.DockerService{Client: cli})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d.EnsureStream(ctx, f.container)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for slices.Contains(vars.ActiveStreams(), f.container) {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			d.StopStream(f.container)
			f.t.Fatal("stream did not finish before deadline")
		}
	}
}

func TestLogsSharingATimestampSurviveAnOutageAndReattach(t *testing.T) {
	f := newDeliveryFixture(t)
	var outage atomic.Bool
	outage.Store(true)
	f.serve(func(w http.ResponseWriter, r *http.Request) {
		item := f.decode(r)
		if outage.Load() && !strings.HasPrefix(item.LogLine[1], "ONLOGS:") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		f.persist(w, item)
	})
	lines := []string{sharedTS + " first-at-same-time", sharedTS + " second-at-same-time", newerTS + " newer-marker"}
	f.stream(lines...)
	if pending := f.messages("brokenlogs", util.GetHost()); len(pending) != 3 {
		t.Fatalf("expected all three undelivered lines queued, got %q", pending)
	}

	outage.Store(false)
	agent.TryResend()
	f.stream(lines...)

	rows := f.messages("logs", f.remoteHost)
	for _, want := range []string{"first-at-same-time", "second-at-same-time", "newer-marker"} {
		if !slices.Contains(rows, want) {
			t.Errorf("%q was lost; stored rows: %q", want, rows)
		}
	}
	if pending := f.messages("brokenlogs", util.GetHost()); len(pending) != 0 {
		t.Errorf("queue not drained: %q", pending)
	}
}

func TestResendKeepsALogQueuedWhileItsTimestampTwinWasInFlight(t *testing.T) {
	f := newDeliveryFixture(t)
	f.serve(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	if agent.SendLogMessage(f.token, f.container, []string{sharedTS, "old-pending"}) {
		t.Fatal("expected initial failure")
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	f.serve(func(w http.ResponseWriter, r *http.Request) {
		item := f.decode(r)
		if item.LogLine[1] == "new-pending" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		f.persist(w, item)
		close(entered)
		<-release
	})
	done := make(chan struct{})
	go func() { agent.TryResend(); close(done) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("retry never reached receiver")
	}
	if agent.SendLogMessage(f.token, f.container, []string{sharedTS, "new-pending"}) {
		close(release)
		t.Fatal("expected new send failure")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("retry did not finish")
	}

	pending := f.messages("brokenlogs", util.GetHost())
	if !slices.Equal(pending, []string{"new-pending"}) {
		t.Fatalf("expected only the newer failed send still queued, got %q", pending)
	}
	if rows := f.messages("logs", f.remoteHost); !slices.Equal(rows, []string{"old-pending"}) {
		t.Fatalf("expected the retried log stored once, got %q", rows)
	}
}

func TestLostAcknowledgementStoresTheEventOnce(t *testing.T) {
	f := newDeliveryFixture(t)
	var requests atomic.Int32
	f.serve(func(w http.ResponseWriter, r *http.Request) {
		item := f.decode(r)
		if requests.Add(1) == 1 {
			rec := httptest.NewRecorder()
			f.persist(rec, item)
			if rec.Code != http.StatusOK {
				panic(fmt.Sprintf("receiver rejected request: %d", rec.Code))
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				panic(err)
			}
			_ = conn.Close()
			return
		}
		f.persist(w, item)
	})
	if agent.SendLogMessage(f.token, f.container, []string{sharedTS, "one-event"}) {
		t.Fatal("transport failure was not exercised")
	}
	agent.TryResend()

	if n := requests.Load(); n != 2 {
		t.Fatalf("expected one retry, got %d requests", n)
	}
	if rows := f.messages("logs", f.remoteHost); !slices.Equal(rows, []string{"one-event"}) {
		t.Fatalf("expected one stored row, got %q", rows)
	}
	if pending := f.messages("brokenlogs", util.GetHost()); len(pending) != 0 {
		t.Fatalf("acknowledged retry left %q queued", pending)
	}
}

func TestResendDeliversAnEntryQueuedUnderTheOldTimestampKey(t *testing.T) {
	f := newDeliveryFixture(t)
	buffer := util.GetDB(util.GetHost(), f.container, "brokenlogs")
	if buffer == nil {
		t.Fatal("could not open the queue")
	}
	if err := buffer.Put([]byte(sharedTS), []byte("queued-by-an-older-agent"), nil); err != nil {
		t.Fatal(err)
	}
	f.serve(func(w http.ResponseWriter, r *http.Request) { f.persist(w, f.decode(r)) })

	agent.TryResend()

	logs := util.GetDB(f.remoteHost, f.container, "logs")
	if logs == nil {
		t.Fatal("could not open the receiver's logs")
	}
	it := logs.NewIterator(nil, nil)
	defer it.Release()
	if !it.Next() || string(it.Value()) != "queued-by-an-older-agent" || !strings.HasPrefix(string(it.Key()), sharedTS+" +") {
		t.Fatal("the old queue entry was not stored at its timestamp")
	}
	if pending := f.messages("brokenlogs", util.GetHost()); len(pending) != 0 {
		t.Fatalf("queue not drained: %q", pending)
	}
}

func TestAgentRestartDoesNotResendTheLineAtItsCursor(t *testing.T) {
	f := newDeliveryFixture(t)
	f.serve(func(w http.ResponseWriter, r *http.Request) { f.persist(w, f.decode(r)) })

	f.stream(sharedTS + " INFO delivered")
	f.stream(sharedTS+" INFO delivered", sharedTS+" INFO new-at-the-same-time")
	f.stream(sharedTS+" INFO delivered", sharedTS+" INFO new-at-the-same-time")

	rows := f.messages("logs", f.remoteHost)
	for _, want := range []string{"INFO delivered", "INFO new-at-the-same-time"} {
		if n := countOf(rows, want); n != 1 {
			t.Errorf("%q stored %d times; rows: %q", want, n, rows)
		}
	}
	if n := countOf(f.messages("statuses", f.remoteHost), "info"); n != 2 {
		t.Errorf("expected 2 info statuses, got %d", n)
	}
	if pending := f.messages("brokenlogs", util.GetHost()); len(pending) != 0 {
		t.Errorf("nothing should be queued: %q", pending)
	}
}

func TestAgentRestartDoesNotResendANonUTF8LineAtItsCursor(t *testing.T) {
	f := newDeliveryFixture(t)
	var sends atomic.Int32
	f.serve(func(w http.ResponseWriter, r *http.Request) {
		if item := f.decode(r); !strings.HasPrefix(item.LogLine[1], "ONLOGS:") {
			sends.Add(1)
		}
	})

	line := sharedTS + " INFO caf\xe9 latin1"
	f.stream(line)
	f.stream(line)

	if n := sends.Load(); n != 1 {
		t.Fatalf("expected 1 send, got %d", n)
	}
}

func countOf(values []string, want string) int {
	n := 0
	for _, v := range values {
		if v == want {
			n++
		}
	}
	return n
}
