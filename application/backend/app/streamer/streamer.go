package streamer

import (
	"context"
	"fmt"
	"os"
	"strconv"
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

	statsMu      sync.Mutex
	statsCancels map[string]context.CancelFunc
}

func getStatsWorkerKey(host, container string) string {
	return host + "/" + container
}

func (ctrl *StreamController) registerStatisticsWorker(location string, cancel context.CancelFunc) bool {
	ctrl.statsMu.Lock()
	defer ctrl.statsMu.Unlock()
	if ctrl.statsCancels == nil {
		ctrl.statsCancels = map[string]context.CancelFunc{}
	}
	if _, exists := ctrl.statsCancels[location]; exists {
		return false
	}
	ctrl.statsCancels[location] = cancel
	return true
}

func (ctrl *StreamController) unregisterStatisticsWorker(location string) (context.CancelFunc, bool) {
	ctrl.statsMu.Lock()
	defer ctrl.statsMu.Unlock()
	cancel, exists := ctrl.statsCancels[location]
	if exists {
		delete(ctrl.statsCancels, location)
	}
	return cancel, exists
}

func (ctrl *StreamController) ensureStatisticsWorker(ctx context.Context, host, container string) {
	location := getStatsWorkerKey(host, container)
	workerCtx, cancel := context.WithCancel(ctx)
	if !ctrl.registerStatisticsWorker(location, cancel) {
		cancel()
		return
	}
	go statistics.RunStatisticForContainerWithContext(workerCtx, host, container)
}

func (ctrl *StreamController) stopStatisticsWorker(host, container string) {
	cancel, exists := ctrl.unregisterStatisticsWorker(getStatsWorkerKey(host, container))
	if exists {
		cancel()
	}
}

func (ctrl *StreamController) statisticsWorkersCount() int {
	ctrl.statsMu.Lock()
	defer ctrl.statsMu.Unlock()
	return len(ctrl.statsCancels)
}

func (ctrl *StreamController) ensureStreams(ctx context.Context, containers []string) {
	host := util.GetHost()
	for _, container := range containers {
		ctrl.ensureStatisticsWorker(ctx, host, container)
		ctrl.DaemonService.EnsureStream(ctx, container)
	}
}

func (ctrl *StreamController) reconcileStreams(ctx context.Context) {
	current := map[string]struct{}{}
	for _, container := range vars.DockerContainers {
		current[container] = struct{}{}
	}

	ctrl.ensureStreams(ctx, vars.DockerContainers)

	for _, active := range append([]string{}, vars.Active_Daemon_Streams...) {
		if _, exists := current[active]; !exists {
			ctrl.DaemonService.StopStream(active)
			ctrl.stopStatisticsWorker(util.GetHost(), active)
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
		if !util.Contains(containerName, vars.DockerContainers) {
			vars.DockerContainers = append(vars.DockerContainers, containerName)
		}
		ctrl.ensureStatisticsWorker(ctx, util.GetHost(), containerName)
		ctrl.DaemonService.EnsureStream(ctx, containerName)
	case "die", "stop", "pause":
		ctrl.DaemonService.StopStream(containerName)
	case "destroy":
		ctrl.DaemonService.StopStream(containerName)
		ctrl.stopStatisticsWorker(util.GetHost(), containerName)
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
	if vars.FavsDBErr != nil || vars.StateDBErr != nil || vars.UsersDBErr != nil {
		fmt.Println("ERROR: unable to open leveldb", vars.FavsDBErr, vars.StateDBErr, vars.UsersDBErr)
		return
	}

	vars.DockerContainers = ctrl.DaemonService.GetContainersList(ctx)
	ctrl.reconcileStreams(ctx)
	if os.Getenv("AGENT") != "" {
		agent.SendInitRequest(vars.DockerContainers)
	}

	go ctrl.startEventsLoop(ctx)

	reconcileTicker := time.NewTicker(60 * time.Second)
	defer reconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconcileTicker.C:
			vars.Year = strconv.Itoa(time.Now().UTC().Year())
			vars.DockerContainers = ctrl.DaemonService.GetContainersList(ctx)
			ctrl.reconcileStreams(ctx)
			if os.Getenv("AGENT") != "" {
				agent.SendUpdate(vars.DockerContainers)
				agent.TryResend()
			}
		}
	}
}
