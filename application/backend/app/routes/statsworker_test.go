package routes

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
)

func addAgentLine(t *testing.T, host, svc string) {
	t.Helper()
	stamp := time.Now().Add(-time.Second).UTC().Format("2006-01-02T15:04:05.000000000Z")
	body := map[string]any{"Token": db.CreateOnLogsToken(), "Host": host, "Container": svc, "LogLine": []string{stamp, "INFO agent line"}}
	if status, raw := call(t, testCtrl.AddLogLine, authedRequest(t, "", "POST", "/api/v1/addLogLine", body)); status != http.StatusOK {
		t.Fatalf("addLogLine returned %d: %s", status, raw)
	}
}

func TestDeleteServiceStopsAnAgentContainersWorker(t *testing.T) {
	const host, svc = "workerdelhost", "svc"
	freshHost(t, host, svc)
	t.Cleanup(func() { statistics.StopWorker(host, svc) })
	baseline := statistics.WorkerCount()

	addAgentLine(t, host, svc)
	if got := statistics.WorkerCount(); got != baseline+1 {
		t.Fatalf("the first line started %d workers, want 1", got-baseline)
	}

	req := authedRequest(t, "admin", "POST", "/api/v1/deleteContainer", map[string]string{"Host": host, "Service": svc})
	if status, body := call(t, testCtrl.DeleteContainer, req); status != http.StatusOK {
		t.Fatalf("deleteContainer: %d %s", status, body)
	}

	deadline := time.Now().Add(10 * time.Second)
	for statistics.WorkerCount() != baseline {
		if time.Now().After(deadline) {
			t.Fatalf("onlogs_stats_workers stayed at %d after Delete service, want %d", statistics.WorkerCount(), baseline)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A line stored just before Delete service reaches EnsureWorker after the
// service is gone; that worker would never be stopped.
func TestALateLineDoesNotRestartADeletedServicesWorker(t *testing.T) {
	const host, svc = "workerlatehost", "svc"
	freshHost(t, host, svc)
	t.Cleanup(func() { statistics.StopWorker(host, svc) })
	baseline := statistics.WorkerCount()

	// Open handles keep the write working once the directory is gone.
	util.GetDB(host, svc, "logs")
	util.GetDB(host, svc, "statuses")
	if err := os.RemoveAll("leveldb/hosts/" + host); err != nil {
		t.Fatal(err)
	}

	addAgentLine(t, host, svc)
	if got := statistics.WorkerCount(); got != baseline {
		t.Fatalf("a line for a deleted service left %d workers running", got-baseline)
	}
}

func TestClearLogsKeepsTheWorker(t *testing.T) {
	const host, svc = "workerclearhost", "svc"
	freshHost(t, host, svc)
	t.Cleanup(func() { statistics.StopWorker(host, svc) })
	baseline := statistics.WorkerCount()

	addAgentLine(t, host, svc)
	req := authedRequest(t, "admin", "POST", "/api/v1/deleteContainerLogs", map[string]string{"host": host, "service": svc})
	if status, body := call(t, testCtrl.DeleteContainerLogs, req); status != http.StatusOK {
		t.Fatalf("deleteContainerLogs: %d %s", status, body)
	}

	if got := statistics.WorkerCount(); got != baseline+1 {
		t.Fatalf("%d workers after Clear logs, want 1", got-baseline)
	}
	if statistics.EnsureWorker(context.Background(), host, svc) {
		t.Fatal("Clear logs stopped the worker")
	}
}

// A line that lands between the delete and the stop recreates the service; the
// stop must not leave it without a worker.
func TestALineDuringDeleteServiceKeepsItsWorker(t *testing.T) {
	const host, svc = "workerdelracehost", "svc"
	freshHost(t, host, svc)
	t.Cleanup(func() { statistics.StopWorker(host, svc) })
	addAgentLine(t, host, svc)

	lineStatus := make(chan int, 1)
	stopped := make(chan struct{})
	stopServiceWorker = func(h, s string) {
		defer close(stopped)
		go func() {
			stamp := time.Now().UTC().Format("2006-01-02T15:04:05.000000000Z")
			body := map[string]any{"Token": db.CreateOnLogsToken(), "Host": host, "Container": svc, "LogLine": []string{stamp, "INFO mid-delete line"}}
			status, _ := call(t, testCtrl.AddLogLine, authedRequest(t, "", "POST", "/api/v1/addLogLine", body))
			lineStatus <- status
		}()
		// Unfixed, the line finishes here; fixed, it waits for the delete to end.
		select {
		case status := <-lineStatus:
			lineStatus <- status
		case <-time.After(500 * time.Millisecond):
		}
		statistics.StopWorker(h, s)
	}
	t.Cleanup(func() { stopServiceWorker = statistics.StopWorker })

	req := authedRequest(t, "admin", "POST", "/api/v1/deleteContainer", map[string]string{"Host": host, "Service": svc})
	if status, body := call(t, testCtrl.DeleteContainer, req); status != http.StatusOK {
		t.Fatalf("deleteContainer: %d %s", status, body)
	}
	<-stopped
	if status := <-lineStatus; status != http.StatusOK {
		t.Fatalf("addLogLine returned %d", status)
	}

	if _, err := os.Stat("leveldb/hosts/" + host + "/containers/" + svc); err != nil {
		t.Fatalf("the line did not recreate the service: %v", err)
	}
	if statistics.EnsureWorker(context.Background(), host, svc) {
		t.Fatal("the recreated service was left without a statistics worker")
	}
}
