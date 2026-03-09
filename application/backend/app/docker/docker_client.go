package docker

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type DockerService struct {
	Client *client.Client
}

type ContainerNamesResult struct {
	Name string
	ID   string
}

func (s *DockerService) GetContainerNames(ctx context.Context) ([]ContainerNamesResult, error) {
	containers, err := s.Client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var res []ContainerNamesResult
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		res = append(res, ContainerNamesResult{Name: name, ID: c.ID})
	}
	return res, nil
}

func (s *DockerService) GetContainerImageNameByContainerID(ctx context.Context, containerID string) (string, error) {
	res, err := s.Client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	return res.Config.Image, nil
}

func (s *DockerService) GetContainerEvents(ctx context.Context) (<-chan events.Message, <-chan error) {
	eventFilters := filters.NewArgs()
	eventFilters.Add("type", "container")
	return s.Client.Events(ctx, events.ListOptions{Filters: eventFilters})
}
