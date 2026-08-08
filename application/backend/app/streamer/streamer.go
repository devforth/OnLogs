package streamer

import (
	"context"
	"fmt"
	"strings"
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
}

func (ctrl *StreamController) ensureStreams(ctx context.Context, containers []string) {
	host := util.GetHost()
	for _, container := range containers {
		statistics.EnsureWorker(ctx, host, container)
		ctrl.DaemonService.EnsureStream(ctx, container)
	}
}

func (ctrl *StreamController) reconcileStreams(ctx context.Context) {
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

	switch msg.Action {
	case "start", "restart", "unpause":
		vars.AddDockerContainer(containerName)
		statistics.EnsureWorker(ctx, util.GetHost(), containerName)
		ctrl.DaemonService.EnsureStream(ctx, containerName)
	case "die", "stop", "pause":
		ctrl.DaemonService.StopStream(containerName)
	case "destroy":
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

	vars.SetDockerContainers(ctrl.DaemonService.GetContainersList(ctx))
	ctrl.reconcileStreams(ctx)
	if util.IsAgentMode() {
		agent.SendInitRequest(vars.DockerContainerList())
	}

	go ctrl.startEventsLoop(ctx)

	reconcileTicker := time.NewTicker(60 * time.Second)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcileTicker.C:
			vars.SetDockerContainers(ctrl.DaemonService.GetContainersList(ctx))
			ctrl.reconcileStreams(ctx)
			if util.IsAgentMode() {
				agent.SendUpdate(vars.DockerContainerList())
				agent.TryResend()
			}
		}
	}
}
