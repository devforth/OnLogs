package streamer

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

var reconcileRuns atomic.Int64

func TestReconcileStopsAStreamOnlyOnAListRequestedAfterItsStart(t *testing.T) {
	t.Setenv("AGENT", "false")
	t.Setenv("MAX_LOGS_SIZE", "10GB")
	name := fmt.Sprintf("started-during-list-%d", reconcileRuns.Add(1))
	attached := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.multiplexed-stream")
		w.(http.Flusher).Flush()
		close(attached)
		<-r.Context().Done()
	}))
	cli, err := client.NewClientWithOpts(client.WithHost(srv.URL), client.WithVersion("1.45"))
	if err != nil {
		t.Fatal(err)
	}
	ctrl := &StreamController{DaemonService: daemon.NewDaemonService(&docker.DockerService{Client: cli})}
	ctx := t.Context()
	t.Cleanup(func() {
		srv.Close()
		_ = cli.Close()
	})

	since := vars.ContainerEventSeq()
	ctrl.handleContainerEvent(ctx, events.Message{Action: "start", Actor: events.Actor{Attributes: map[string]string{"name": name}}})
	<-attached
	vars.SetDockerContainers(nil, since)
	ctrl.reconcileStreams(ctx)
	if !slices.Contains(vars.ActiveStreams(), name) {
		t.Fatal("reconcile stopped the stream of a container that started while the list was in flight")
	}
	if statistics.EnsureWorker(ctx, util.GetHost(), name) {
		t.Fatal("reconcile stopped the statistics worker of a container that started while the list was in flight")
	}

	vars.SetDockerContainers(nil, vars.ContainerEventSeq())
	ctrl.reconcileStreams(ctx)
	if slices.Contains(vars.ActiveStreams(), name) {
		t.Fatal("a list requested after the start still kept the stream of a container missing from it")
	}
	if !statistics.EnsureWorker(ctx, util.GetHost(), name) {
		t.Fatal("a list requested after the start still kept the statistics worker of a container missing from it")
	}
	statistics.StopWorker(util.GetHost(), name)
}
