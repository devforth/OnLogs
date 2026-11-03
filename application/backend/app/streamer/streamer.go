package streamer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/api/types/events"
)

type StreamController struct {
	DaemonService *daemon.DaemonService

	// Keeps a container event from landing between reconcile's list read and its stops.
	mu sync.Mutex
}

func (ctrl *StreamController) ensureStreams(ctx context.Context, containers []string) {
	host := util.GetHost()
	for _, container := range containers {
		statistics.EnsureWorker(ctx, host, container)
		ctrl.DaemonService.EnsureStream(ctx, container)
	}
}

func (ctrl *StreamController) reconcileStreams(ctx context.Context) {
	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	containers := vars.DockerContainerList()
	current := map[string]struct{}{}
	for _, container := range containers {
		current[container] = struct{}{}
	}

	ctrl.ensureStreams(ctx, containers)

	for _, active := range vars.ActiveStreams() {
		if _, exists := current[active]; !exists {
			ctrl.DaemonService.StopStream(active)
			statistics.StopWorker(util.GetHost(), active)
		}
	}
}

func (ctrl *StreamController) handleContainerEvent(ctx context.Context, msg events.Message) {
	containerName := strings.TrimPrefix(msg.Actor.Attributes["name"], "/")
	if containerName == "" {
		return
	}

	ctrl.mu.Lock()
	defer ctrl.mu.Unlock()
	switch msg.Action {
	case "rename":
		oldName := strings.TrimPrefix(msg.Actor.Attributes["oldName"], "/")
		// Delete Docker logs looks the id up by name.
		if meta := util.GetContainersMetaDB(util.GetHost()); meta != nil {
			_ = meta.Delete([]byte(oldName), nil)
			_ = meta.Put([]byte(containerName), []byte(msg.Actor.ID), nil)
		}
		if !ctrl.DaemonService.StopRenamedStream(oldName, containerName) {
			return
		}
		vars.RemoveDockerContainer(oldName)
		statistics.StopWorker(util.GetHost(), oldName)
		fallthrough
	case "start", "restart", "unpause":
		vars.AddDockerContainer(containerName)
		statistics.EnsureWorker(ctx, util.GetHost(), containerName)
		ctrl.DaemonService.EnsureStream(ctx, containerName)
	case "die", "stop", "pause":
		vars.RemoveDockerContainer(containerName)
		ctrl.DaemonService.StopStream(containerName)
	case "destroy":
		vars.RemoveDockerContainer(containerName)
		ctrl.DaemonService.StopStream(containerName)
		statistics.StopWorker(util.GetHost(), containerName)
	}
}

func (ctrl *StreamController) startEventsLoop(ctx context.Context) {
	for {
		eventCtx, cancel := context.WithCancel(ctx)
		eventsCh, errsCh := ctrl.DaemonService.DockerClient.GetContainerEvents(eventCtx)
		shouldRetry := false

		for !shouldRetry {
			select {
			case <-ctx.Done():
				cancel()
				return
			case msg, ok := <-eventsCh:
				if !ok {
					shouldRetry = true
					break
				}
				ctrl.handleContainerEvent(ctx, msg)
			case err, ok := <-errsCh:
				if !ok {
					shouldRetry = true
					break
				}
				if err != nil && err != context.Canceled {
					fmt.Println("WARN: docker events stream interrupted:", err)
					shouldRetry = true
				}
			}
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

func (ctrl *StreamController) StreamLogs(ctx context.Context) {
	if vars.FavsDBErr != nil || vars.UsersDBErr != nil {
		fmt.Println("ERROR: unable to open leveldb", vars.FavsDBErr, vars.UsersDBErr)
		return
	}

	since := vars.ContainerEventSeq()
	vars.SetDockerContainers(ctrl.DaemonService.GetContainersList(ctx), since)
	ctrl.reconcileStreams(ctx)
	// Started first so containers keep streaming while the agent waits for the master.
	go ctrl.startEventsLoop(ctx)
	if util.IsAgentMode() {
		if err := agent.SendInitRequest(ctx, vars.DockerContainerList); err != nil {
			if ctx.Err() != nil {
				return
			}
			fmt.Println("FATAL:", err)
			os.Exit(1)
		}
	}

	reconcileTicker := time.NewTicker(60 * time.Second)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcileTicker.C:
			since := vars.ContainerEventSeq()
			vars.SetDockerContainers(ctrl.DaemonService.GetContainersList(ctx), since)
			ctrl.reconcileStreams(ctx)
			if util.IsAgentMode() {
				agent.SendUpdate(vars.DockerContainerList())
				agent.TryResend()
			}
		}
	}
}
