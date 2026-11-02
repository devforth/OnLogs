package agent_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/routes"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

type deleteFixture struct {
	t         *testing.T
	host, dir string
	lose      atomic.Bool
}

func newDeleteFixture(t *testing.T) *deleteFixture {
	t.Helper()
	t.Chdir(t.TempDir())
	f := &deleteFixture{t: t, host: util.GetHost(), dir: t.TempDir() + "/"}
	dockerDir := util.DockerContainersDir
	util.DockerContainersDir = f.dir
	token := db.CreateOnLogsToken()
	t.Setenv("ONLOGS_TOKEN", token)
	closeMeta := func() {
		vars.DBMutex.Lock()
		defer vars.DBMutex.Unlock()
		if d := vars.ContainersMeta_DBs[f.host]; d != nil {
			d.Close()
			delete(vars.ContainersMeta_DBs, f.host)
		}
	}
	closeMeta()
	vars.TakeQueuedDeletes(f.host)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !f.lose.CompareAndSwap(true, false) {
			(&routes.RouteController{}).AskForDelete(w, r)
			return
		}
		(&routes.RouteController{}).AskForDelete(httptest.NewRecorder(), r)
		if conn, _, err := w.(http.Hijacker).Hijack(); err == nil {
			conn.Close()
		}
	}))
	t.Setenv("HOST", server.URL)
	t.Cleanup(func() {
		server.Close()
		util.DockerContainersDir = dockerDir
		closeMeta()
		vars.TakeQueuedDeletes(f.host)
		_ = vars.TokensDB.Delete([]byte(token), nil)
	})
	return f
}

func (f *deleteFixture) container(name, content string) string {
	f.t.Helper()
	id := name + "-id"
	if err := util.GetContainersMetaDB(f.host).Put([]byte(name), []byte(id), nil); err != nil {
		f.t.Fatal(err)
	}
	if err := os.MkdirAll(f.dir+id, 0o700); err != nil {
		f.t.Fatal(err)
	}
	path := f.dir + id + "/" + id + "-json.log"
	f.write(path, content)
	return path
}

func (f *deleteFixture) write(path, content string) {
	f.t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		f.t.Fatal(err)
	}
}

func (f *deleteFixture) read(path string) string {
	f.t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func captureStdout(t *testing.T, run func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	output := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		output <- string(data)
	}()
	run()
	os.Stdout = stdout
	w.Close()
	return <-output
}

func TestDeleteIsRedeliveredWhenThePollResponseIsLost(t *testing.T) {
	f := newDeleteFixture(t)
	log := f.container("target", "old output\n")
	vars.QueueDelete(f.host, "target")

	f.lose.Store(true)
	agent.AskForDelete()
	if got := f.read(log); got != "old output\n" {
		t.Fatalf("the log changed although the poll response was lost: %q", got)
	}

	agent.AskForDelete()
	if got := f.read(log); got != "" {
		t.Fatalf("the deletion was not redelivered after the lost response: %q", got)
	}

	f.write(log, "newer output\n")
	agent.AskForDelete()
	if got := f.read(log); got != "newer output\n" {
		t.Fatalf("an acknowledged deletion was applied again: %q", got)
	}
}

func TestFailedDeleteIsRetriedOnTheNextPoll(t *testing.T) {
	f := newDeleteFixture(t)
	log := f.container("target", "")
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(log, 0o700); err != nil {
		t.Fatal(err)
	}
	vars.QueueDelete(f.host, "target")

	agent.AskForDelete()

	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	f.write(log, "old output\n")
	agent.AskForDelete()
	if got := f.read(log); got != "" {
		t.Fatalf("the failed deletion was not retried: %q", got)
	}

	f.write(log, "newer output\n")
	agent.AskForDelete()
	if got := f.read(log); got != "newer output\n" {
		t.Fatalf("a completed deletion was applied again: %q", got)
	}
}

func TestDeleteThatKeepsFailingIsGivenUpAndReported(t *testing.T) {
	f := newDeleteFixture(t)
	vars.QueueDelete(f.host, "missing")

	output := captureStdout(t, func() {
		for range 4 {
			agent.AskForDelete()
		}
	})

	if n := strings.Count(output, "unable to delete docker logs of missing:"); n != 3 {
		t.Errorf("the agent tried the deletion %d times, want 3:\n%s", n, output)
	}
	report := fmt.Sprintf("gave up deleting docker logs of missing on %s:", f.host)
	if !strings.Contains(output, report) || !strings.Contains(output, "no docker container id for missing") {
		t.Errorf("the master did not report the failed deletion with its error:\n%s", output)
	}
	if left := vars.TakeQueuedDeletes(f.host); len(left) != 0 {
		t.Errorf("the master still holds the abandoned deletion: %v", left)
	}
}

func TestDeletesStillReachAgentsAndMastersThatDoNotAcknowledge(t *testing.T) {
	t.Run("older agent", func(t *testing.T) {
		f := newDeleteFixture(t)
		vars.QueueDelete(f.host, "target")
		poll := func() []string {
			body, _ := json.Marshal(map[string]string{"Hostname": f.host, "Token": os.Getenv("ONLOGS_TOKEN")})
			resp, err := http.Post(os.Getenv("HOST")+"/api/v1/askForDelete", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var reply map[string][]string
			if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
				t.Fatal(err)
			}
			return reply["Services"]
		}
		if got := poll(); !slices.Equal(got, []string{"target"}) {
			t.Fatalf("an agent that does not acknowledge got %v, want [target]", got)
		}
		if got := poll(); len(got) != 0 {
			t.Fatalf("an agent that does not acknowledge got the deletion again: %v", got)
		}
	})

	t.Run("older master", func(t *testing.T) {
		f := newDeleteFixture(t)
		log := f.container("target", "old output\n")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"Services":["target"]}`)
		}))
		t.Cleanup(server.Close)
		t.Setenv("HOST", server.URL)

		agent.AskForDelete()
		if got := f.read(log); got != "" {
			t.Fatalf("a deletion from a master without command ids was not applied: %q", got)
		}
	})
}

func TestDockerLogsOfARunningContainerAreKept(t *testing.T) {
	f := newDeleteFixture(t)
	log := f.container("target", "old output\n")
	vars.SetDockerContainers([]string{"target"}, vars.ContainerEventSeq())
	vars.SetAgentContainers("edge", []string{"remote"})
	t.Cleanup(func() {
		vars.SetDockerContainers(nil, vars.ContainerEventSeq())
		vars.SetAgentContainers("edge", nil)
		vars.TakeQueuedDeletes("edge")
	})

	if err := util.DeleteDockerLogs(f.host, "target"); err == nil {
		t.Error("a running container's docker log was cleared without an error")
	}
	if got := f.read(log); got != "old output\n" {
		t.Errorf("a running container's docker log was cleared: %q", got)
	}
	if err := util.DeleteDockerLogs("edge", "remote"); err == nil {
		t.Error("a running agent container's docker log was queued without an error")
	}
	if queued := vars.TakeQueuedDeletes("edge"); len(queued) != 0 {
		t.Errorf("a running agent container's docker log was queued: %v", queued)
	}
}
