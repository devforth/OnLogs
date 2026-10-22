package vars

import (
	"slices"
	"sync"
	"testing"
	"time"
)

func TestSharedStateSurvivesConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	stop := make(chan struct{})

	writers := []func(){
		func() { AddActiveStream("container") },
		func() { RemoveActiveStream("container") },
		func() { AddDockerContainer("container") },
		func() { RemoveDockerContainer("container") },
		func() { SetDockerContainers([]string{"a", "b"}, ContainerEventSeq()) },
		func() { SetAgentContainers("host", []string{"a"}) },
		func() { QueueDelete("host", "container") },
	}
	readers := []func(){
		func() { _ = ActiveStreams() },
		func() { _ = DockerContainerList() },
		func() { _ = AgentContainers("host") },
		func() { _ = TakeQueuedDeletes("host") },
		func() { _ = PendingDeletes("host", []string{}, nil) },
	}

	for _, fn := range append(append([]func(){}, writers...), readers...) {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				f()
			}
		}(fn)
	}

	time.Sleep(120 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestQueuedDeletesAreKeptPerHostAndDrainedOnce(t *testing.T) {
	host := "queuehost"
	_ = TakeQueuedDeletes(host)

	QueueDelete(host, "first")
	QueueDelete(host, "second")

	queued := TakeQueuedDeletes(host)
	if len(queued) != 2 || queued[0] != "first" || queued[1] != "second" {
		t.Fatalf("queued deletions were dropped: %v", queued)
	}
	if again := TakeQueuedDeletes(host); len(again) != 0 {
		t.Fatalf("the queue was not drained: %v", again)
	}
}

func TestDockerContainersKeepEventsNewerThanTheList(t *testing.T) {
	SetDockerContainers(nil, ContainerEventSeq())
	AddDockerContainer("started-before-request")
	since := ContainerEventSeq()
	AddDockerContainer("started")
	AddDockerContainer("short-job")
	RemoveDockerContainer("short-job")
	RemoveDockerContainer("died")

	SetDockerContainers([]string{"listed", "died"}, since)
	got := DockerContainerList()
	slices.Sort(got)
	if !slices.Equal(got, []string{"listed", "started"}) {
		t.Fatalf("a list requested before the events undid them: %v", got)
	}

	SetDockerContainers([]string{"listed"}, ContainerEventSeq())
	if got := DockerContainerList(); !slices.Equal(got, []string{"listed"}) {
		t.Fatalf("a list requested after the events did not replace them: %v", got)
	}
}
