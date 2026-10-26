package streamer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

type fakeLine struct {
	ts   time.Time
	text string
}

type fakeContainer struct {
	id      string
	name    string
	running bool
	lines   []fakeLine
}

type fakeDocker struct {
	mu         sync.Mutex
	cond       *sync.Cond
	containers []*fakeContainer
	last       time.Time
}

func (f *fakeDocker) add(name string) *fakeContainer {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := &fakeContainer{id: fmt.Sprintf("%064d", len(f.containers)+1), name: name, running: true}
	f.containers = append(f.containers, c)
	return c
}

func (f *fakeDocker) update(change func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change()
	f.cond.Broadcast()
}

// next is later than every timestamp handed out before; f.mu must be held.
func (f *fakeDocker) next() time.Time {
	ts := time.Now()
	if !ts.After(f.last) {
		ts = f.last.Add(time.Microsecond)
	}
	f.last = ts
	return ts
}

func (f *fakeDocker) stamp() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.next()
}

func (f *fakeDocker) write(c *fakeContainer, text string) {
	f.update(func() { c.lines = append(c.lines, fakeLine{f.next(), text}) })
}

func (f *fakeDocker) logs(w http.ResponseWriter, r *http.Request) {
	ref := path.Base(path.Dir(r.URL.Path))
	sec, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	f.mu.Lock()
	var c *fakeContainer
	for _, candidate := range f.containers {
		if candidate.name == ref {
			c = candidate
		}
	}
	f.mu.Unlock()
	if c == nil {
		http.Error(w, `{"message":"No such container"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.docker.raw-stream")
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()
	defer context.AfterFunc(r.Context(), func() { f.update(func() {}) })()

	for next := 0; ; {
		f.mu.Lock()
		for next == len(c.lines) && c.running && r.Context().Err() == nil {
			f.cond.Wait()
		}
		pending, done := c.lines[next:], !c.running || r.Context().Err() != nil
		next = len(c.lines)
		f.mu.Unlock()
		for _, l := range pending {
			if !l.ts.Before(time.Unix(sec, 0)) {
				fmt.Fprintf(w, "%s %s\n", l.ts.UTC().Format(time.RFC3339Nano), l.text)
			}
		}
		w.(http.Flusher).Flush()
		if done {
			return
		}
	}
}

func containerEvent(action string, c *fakeContainer, attrs ...string) events.Message {
	attributes := map[string]string{"name": c.name}
	for i := 0; i+1 < len(attrs); i += 2 {
		attributes[attrs[i]] = attrs[i+1]
	}
	return events.Message{Type: events.ContainerEventType, Action: events.Action(action), Actor: events.Actor{ID: c.id, Attributes: attributes}}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func storedCount(name, text string) int {
	iter := util.GetDB(util.GetHost(), name, "logs").NewIterator(nil, nil)
	defer iter.Release()
	n := 0
	for iter.Next() {
		if string(iter.Value()) == text {
			n++
		}
	}
	return n
}

func waitStored(t *testing.T, name string, texts ...string) {
	t.Helper()
	for _, text := range texts {
		waitFor(t, fmt.Sprintf("%q stored under %s", text, name), func() bool { return storedCount(name, text) > 0 })
	}
}

func storedUnderEither(name1, name2, text string) int {
	return storedCount(name1, text) + storedCount(name2, text)
}

func newRenameFixture(t *testing.T, names ...string) (*fakeDocker, *StreamController, context.Context) {
	t.Chdir(t.TempDir())
	t.Setenv("AGENT", "false")
	t.Setenv("MAX_LOGS_SIZE", "10GB")
	fd := &fakeDocker{}
	fd.cond = sync.NewCond(&fd.mu)
	srv := httptest.NewServer(http.HandlerFunc(fd.logs))
	// A cancel racing a kept-alive response's end can strand the body read.
	srv.Config.SetKeepAlivesEnabled(false)
	cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.45"))
	if err != nil {
		t.Fatal(err)
	}
	ctrl := &StreamController{DaemonService: daemon.NewDaemonService(&docker.DockerService{Client: cli})}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		fd.update(func() {
			for _, c := range fd.containers {
				c.running = false
			}
		})
		waitFor(t, "streams released", func() bool { return len(vars.ActiveStreams()) == 0 })
		cancel()
		waitFor(t, "statistics workers released", func() bool { return statistics.WorkerCount() == 0 })
		_ = cli.Close()
		srv.Close()
		for _, name := range names {
			for _, kind := range []string{"logs", "streamstate", "statistics", "statuses"} {
				util.ResetDB(util.GetHost(), name, kind)
			}
		}
	})
	return fd, ctrl, ctx
}

func TestRenameKeepsEachContainerStreamedUnderItsOwnName(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "bg", "bg-old")
	oldC := fd.add("bg")
	ctrl.handleContainerEvent(ctx, containerEvent("start", oldC))
	fd.write(oldC, "old-before")
	waitStored(t, "bg", "old-before")

	fd.update(func() { oldC.name = "bg-old" })
	newC := fd.add("bg")
	fd.write(newC, "new-0")
	fd.write(oldC, "old-after-0")
	waitStored(t, "bg", "old-after-0")

	ctrl.handleContainerEvent(ctx, containerEvent("rename", oldC, "oldName", "/bg"))
	ctrl.handleContainerEvent(ctx, containerEvent("start", newC))
	fd.write(newC, "new-1")
	fd.write(oldC, "old-after-1")

	waitStored(t, "bg", "new-0", "new-1")
	waitStored(t, "bg-old", "old-after-1")
	for _, text := range []string{"new-0", "new-1"} {
		if got := storedCount("bg", text); got != 1 {
			t.Errorf("%q stored %d times under the reused name", text, got)
		}
	}
	if got := storedCount("bg", "old-after-1"); got != 0 {
		t.Errorf("the renamed container's line was stored under its old name %d times", got)
	}
}

func TestReplacementExitDoesNotStopTheRenamedContainersStream(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "swap", "swap-old")
	oldC := fd.add("swap")
	ctrl.handleContainerEvent(ctx, containerEvent("start", oldC))
	fd.write(oldC, "old-before")
	waitStored(t, "swap", "old-before")

	fd.update(func() { oldC.name = "swap-old" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", oldC, "oldName", "/swap"))
	newC := fd.add("swap")
	ctrl.handleContainerEvent(ctx, containerEvent("start", newC))
	fd.write(newC, "new-early")
	waitStored(t, "swap", "new-early")

	fd.update(func() { newC.running = false })
	ctrl.handleContainerEvent(ctx, containerEvent("die", newC))
	fd.write(oldC, "old-after-die")
	waitStored(t, "swap-old", "old-after-die")

	fd.update(func() { newC.running = true })
	ctrl.handleContainerEvent(ctx, containerEvent("start", newC))
	fd.write(newC, "new-late")
	waitStored(t, "swap", "new-late")
	for _, text := range []string{"new-early", "new-late"} {
		if got := storedCount("swap", text); got != 1 {
			t.Errorf("%q stored %d times", text, got)
		}
	}
	if got := storedCount("swap", "old-after-die"); got != 0 {
		t.Errorf("the renamed container's line was stored under the replacement's name %d times", got)
	}
}

func TestRenameStopsTheOldNamesWorkerEvenOnAListRequestedBeforeIt(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "pr", "pr-new")
	c := fd.add("pr")
	ctrl.handleContainerEvent(ctx, containerEvent("start", c))
	fd.write(c, "before")
	waitStored(t, "pr", "before")

	since := vars.ContainerEventSeq()
	fd.update(func() { c.name = "pr-new" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", c, "oldName", "/pr"))
	vars.SetDockerContainers([]string{"pr"}, since)
	ctrl.reconcileStreams(ctx)
	fd.write(c, "after")
	waitStored(t, "pr-new", "after")
	if statistics.EnsureWorker(ctx, util.GetHost(), "pr-new") {
		t.Error("the new name has no statistics worker")
	}
	if !statistics.EnsureWorker(ctx, util.GetHost(), "pr") {
		t.Error("the old name's statistics worker still runs")
	}
	statistics.StopWorker(util.GetHost(), "pr")
}

func TestRenameOntoAReusedNameKeepsLinesTheOldStreamHadNotRead(t *testing.T) {
	for _, holderDrained := range []bool{true, false} {
		t.Run(fmt.Sprintf("holderDrained=%v", holderDrained), func(t *testing.T) {
			fd, ctrl, ctx := newRenameFixture(t, "lg", "lg-old")
			a, b := fd.add("lg-old"), fd.add("lg")
			ctrl.handleContainerEvent(ctx, containerEvent("start", a))
			ctrl.handleContainerEvent(ctx, containerEvent("start", b))
			fd.write(b, "b-0")
			waitStored(t, "lg", "b-0")
			lagged := fd.stamp()
			fd.write(a, "a-last")
			waitStored(t, "lg-old", "a-last")

			dieA, destroyA := containerEvent("die", a), containerEvent("destroy", a)
			fd.update(func() { a.name, a.running = "removed-a", !holderDrained })
			ctrl.handleContainerEvent(ctx, dieA)
			ctrl.handleContainerEvent(ctx, destroyA)
			if holderDrained {
				waitStored(t, "lg-old", "ONLOGS: Container listening stopped! (EOF)")
			}

			fd.update(func() { b.name = "lg-old" })
			ctrl.handleContainerEvent(ctx, containerEvent("rename", b, "oldName", "/lg"))
			fd.update(func() { b.lines = append(b.lines, fakeLine{lagged, "b-lagged"}) })
			if !holderDrained {
				fd.write(a, "a-drained")
				waitStored(t, "lg-old", "a-drained")
				fd.update(func() { a.running = false })
			}
			fd.write(b, "b-after")
			waitFor(t, "b-after stored", func() bool { return storedUnderEither("lg", "lg-old", "b-after") > 0 })
			if storedUnderEither("lg", "lg-old", "b-lagged") == 0 {
				t.Error("a line the renamed container wrote before the rename was lost")
			}
		})
	}
}

func TestRenameAndBackStoresEarlierLinesOnceUnderTheOriginalName(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "rb", "rb-x")
	c := fd.add("rb")
	ctrl.handleContainerEvent(ctx, containerEvent("start", c))
	fd.write(c, "first")
	waitStored(t, "rb", "first")

	fd.update(func() { c.name = "rb-x" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", c, "oldName", "/rb"))
	fd.write(c, "mid")
	waitStored(t, "rb-x", "mid")

	fd.update(func() { c.name = "rb" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", c, "oldName", "/rb-x"))
	fd.write(c, "last")
	waitStored(t, "rb", "last")
	if got := storedCount("rb", "first"); got != 1 {
		t.Errorf("%q stored %d times under the original name", "first", got)
	}
}

func TestRenameSeenByATickFirstKeepsTheReplacementsLines(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "tr", "tr-old")
	oldC := fd.add("tr")
	ctrl.handleContainerEvent(ctx, containerEvent("start", oldC))
	fd.write(oldC, "old-before")
	waitStored(t, "tr", "old-before")

	since := vars.ContainerEventSeq()
	fd.update(func() { oldC.name = "tr-old" })
	vars.SetDockerContainers([]string{"tr-old"}, since)
	ctrl.reconcileStreams(ctx)
	ctrl.handleContainerEvent(ctx, containerEvent("rename", oldC, "oldName", "/tr"))

	newC := fd.add("tr")
	ctrl.handleContainerEvent(ctx, containerEvent("start", newC))
	fd.write(newC, "new-0")
	fd.write(oldC, "old-after")
	waitStored(t, "tr-old", "old-after")

	fd.update(func() { oldC.running = false })
	ctrl.handleContainerEvent(ctx, containerEvent("die", oldC))
	fd.write(newC, "new-1")
	waitStored(t, "tr", "new-1")
	if got := storedCount("tr", "new-0"); got != 1 {
		t.Errorf("the replacement's first line stored %d times", got)
	}
	if got := storedCount("tr", "old-after"); got != 0 {
		t.Errorf("the renamed container's line was stored under its old name %d times", got)
	}
}

func TestRenameWhilePausedKeepsTheReplacementsLines(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "tp", "tp-old")
	oldC := fd.add("tp")
	ctrl.handleContainerEvent(ctx, containerEvent("start", oldC))
	fd.write(oldC, "old-before")
	waitStored(t, "tp", "old-before")

	ctrl.handleContainerEvent(ctx, containerEvent("pause", oldC))
	fd.update(func() { oldC.name = "tp-old" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", oldC, "oldName", "/tp"))
	newC := fd.add("tp")
	ctrl.handleContainerEvent(ctx, containerEvent("start", newC))
	fd.write(newC, "new-0")
	ctrl.handleContainerEvent(ctx, containerEvent("unpause", oldC))
	fd.write(oldC, "old-after")
	waitStored(t, "tp-old", "old-after")

	fd.update(func() { oldC.running = false })
	ctrl.handleContainerEvent(ctx, containerEvent("die", oldC))
	fd.write(newC, "new-1")
	waitStored(t, "tp", "new-1")
	if got := storedCount("tp", "new-0"); got != 1 {
		t.Errorf("the replacement's first line stored %d times", got)
	}
	if got := storedCount("tp", "old-after"); got != 0 {
		t.Errorf("the renamed container's line was stored under its old name %d times", got)
	}
}

func TestSecondRenameWhileTheReusedNameDrainsKeepsStreaming(t *testing.T) {
	fd, ctrl, ctx := newRenameFixture(t, "sd-a", "sd-b", "sd-c")
	x, a := fd.add("sd-b"), fd.add("sd-a")
	ctrl.handleContainerEvent(ctx, containerEvent("start", x))
	ctrl.handleContainerEvent(ctx, containerEvent("start", a))
	fd.write(a, "a-0")
	waitStored(t, "sd-a", "a-0")
	lagged := fd.stamp()

	die, destroy := containerEvent("die", x), containerEvent("destroy", x)
	fd.update(func() { x.name = "removed-x" })
	ctrl.handleContainerEvent(ctx, die)
	ctrl.handleContainerEvent(ctx, destroy)
	fd.update(func() { a.name = "sd-b" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", a, "oldName", "/sd-a"))
	fd.update(func() { a.name = "sd-c" })
	ctrl.handleContainerEvent(ctx, containerEvent("rename", a, "oldName", "/sd-b"))
	fd.update(func() { a.lines = append(a.lines, fakeLine{lagged, "a-lagged"}) })
	fd.write(a, "a-1")
	waitStored(t, "sd-c", "a-lagged", "a-1")

	fd.update(func() { x.running = false })
	waitStored(t, "sd-b", "ONLOGS: Container listening stopped! (EOF)")
	if slices.Contains(vars.ActiveStreams(), "sd-b") {
		t.Error("the drained name was attached again after the container left it")
	}
}

func TestRestartAfterATickSawTheRenameFirstStoresEachLineOnce(t *testing.T) {
	for _, replacementListed := range []bool{false, true} {
		t.Run(fmt.Sprintf("replacementListed=%v", replacementListed), func(t *testing.T) {
			fd, ctrl, ctx := newRenameFixture(t, "rs", "rs-old")
			c := fd.add("rs")
			ctrl.handleContainerEvent(ctx, containerEvent("start", c))
			fd.write(c, "first")
			waitStored(t, "rs", "first")

			since := vars.ContainerEventSeq()
			fd.update(func() { c.name = "rs-old" })
			listed := []string{"rs-old"}
			if replacementListed {
				fd.add("rs")
				listed = append(listed, "rs")
			}
			vars.SetDockerContainers(listed, since)
			ctrl.reconcileStreams(ctx)
			waitStored(t, "rs-old", "first")
			ctrl.handleContainerEvent(ctx, containerEvent("rename", c, "oldName", "/rs"))
			fd.write(c, "mid")
			waitStored(t, "rs-old", "mid")

			fd.update(func() { c.running = false })
			ctrl.handleContainerEvent(ctx, containerEvent("die", c))
			waitStored(t, "rs-old", "ONLOGS: Container listening stopped! (EOF)")
			fd.update(func() { c.running = true })
			ctrl.handleContainerEvent(ctx, containerEvent("start", c))
			fd.write(c, "last")
			waitStored(t, "rs-old", "last")
			if got := storedCount("rs-old", "mid"); got != 1 {
				t.Errorf("%q stored %d times under the new name", "mid", got)
			}
		})
	}
}
