package streamer

import (
	"context"
	"testing"

	"github.com/devforth/OnLogs/app/daemon"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/docker/docker/client"
)

func initTestConfig() *StreamController {
	cli, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	defer cli.Close()

	dockerService := &docker.DockerService{
		Client: cli,
	}

	daemonService := &daemon.DaemonService{
		DockerClient: dockerService,
	}

    // Initialize the "Controller" with its dependencies
    routerCtrl := &StreamController{
		DaemonService: daemonService,
    }
	return routerCtrl
}

func Test_createStreams(t *testing.T) {
	ctrl := initTestConfig()
	ctrl.DaemonService.CreateDaemonToDBStream(context.TODO(), "logprinter")
}
