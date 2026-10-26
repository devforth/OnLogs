package statistics

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func newServiceWithLine(t *testing.T, host, svc string) string {
	t.Helper()
	dir := "leveldb/hosts/" + host + "/containers/" + svc
	os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() {
		for _, dbType := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, svc, dbType)
		}
		os.RemoveAll("leveldb/hosts/" + host)
		vars.Mutex.Lock()
		delete(vars.Container_Stat_Counter, WorkerKey(host, svc))
		vars.Mutex.Unlock()
	})
	putMetaLine(t, host, svc)
	restartStats(host, svc)
	return dir
}

func putMetaLine(t *testing.T, host, svc string) {
	t.Helper()
	line := []string{time.Now().UTC().Add(-time.Second).Format(fixedWidthNanos), "ONLOGS: Container listening started!"}
	if err := containerdb.PutLogMessage(util.GetDB(host, svc, "logs"), host, svc, line); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshDoesNotRecreateADeletedService(t *testing.T) {
	host, svc := "DeletedHost", "svc"
	dir := newServiceWithLine(t, host, svc)

	containerdb.DeleteContainer(host, svc, true)
	restartStats(host, svc)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("a statistics refresh recreated the deleted service: %v %v", err, entries)
	}
}

func TestStoppedWorkerFinalFlushDoesNotRecreateADeletedService(t *testing.T) {
	host, svc := "StoppedHost", "svc"
	dir := newServiceWithLine(t, host, svc)

	passes := make(chan struct{}, 8)
	SetBeforeStatsReset(func(location string) {
		if location == WorkerKey(host, svc) {
			passes <- struct{}{}
		}
	})
	t.Cleanup(func() { SetBeforeStatsReset(nil) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunStatisticForContainerWithContext(ctx, host, svc)
		close(done)
	}()
	<-passes

	containerdb.DeleteContainer(host, svc, true)
	cancel()
	<-done

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("the stopped worker's final flush recreated the deleted service: %v %v", err, entries)
	}
}

func TestFlushDoesNotRecreateAServiceDeletedBeforeItsWrite(t *testing.T) {
	host, svc := "DeletedMidFlushHost", "svc"
	dir := newServiceWithLine(t, host, svc)
	putMetaLine(t, host, svc)
	t.Cleanup(func() { SetBeforeStatsSave(nil) })
	SetBeforeStatsSave(func(location string) {
		if location == WorkerKey(host, svc) {
			containerdb.DeleteContainer(host, svc, true)
		}
	})

	restartStats(host, svc)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("a flush recreated the service deleted before its write: %v %v", err, entries)
	}
}

func TestClearedServiceCountsLinesLoggedAfterTheClear(t *testing.T) {
	host, svc := "ClearedHost", "svc"
	newServiceWithLine(t, host, svc)

	containerdb.DeleteContainer(host, svc, false)
	putMetaLine(t, host, svc)
	restartStats(host, svc)

	if got := persistedMeta(t, host, svc); got != 1 {
		t.Fatalf("saved %d lines after the clear, want 1", got)
	}
}
