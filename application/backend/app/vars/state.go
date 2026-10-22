package vars

import (
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
)

var stateMutex sync.RWMutex

func ActiveStreams() []string {
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return append([]string{}, Active_Daemon_Streams...)
}

func AddActiveStream(container string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	for _, existing := range Active_Daemon_Streams {
		if existing == container {
			return
		}
	}
	Active_Daemon_Streams = append(Active_Daemon_Streams, container)
}

func RemoveActiveStream(container string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	remaining := make([]string, 0, len(Active_Daemon_Streams))
	for _, existing := range Active_Daemon_Streams {
		if existing != container {
			remaining = append(remaining, existing)
		}
	}
	Active_Daemon_Streams = remaining
}

func DockerContainerList() []string {
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return append([]string{}, DockerContainers...)
}

func SetDockerContainers(containers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	DockerContainers = append([]string{}, containers...)
}

func AddDockerContainer(container string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	for _, existing := range DockerContainers {
		if existing == container {
			return
		}
	}
	DockerContainers = append(DockerContainers, container)
}

func SetAgentContainers(host string, containers []string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	AgentsActiveContainers[host] = append([]string{}, containers...)
}

func AgentContainers(host string) []string {
	stateMutex.RLock()
	defer stateMutex.RUnlock()
	return append([]string{}, AgentsActiveContainers[host]...)
}

const maxDeleteAttempts = 3

type DeleteCommand struct {
	ID        string
	Container string
	failures  int
}

// QueueDelete records a container whose docker logs a remote agent should clear.
func QueueDelete(host string, container string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	ToDelete[host] = append(ToDelete[host], DeleteCommand{ID: rand.Text(), Container: container})
}

// For older agents, which never acknowledge deletes.
func TakeQueuedDeletes(host string) []string {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	queued := []string{}
	for _, command := range ToDelete[host] {
		queued = append(queued, command.Container)
	}
	delete(ToDelete, host)
	return queued
}

func PendingDeletes(host string, done []string, failed map[string]string) []DeleteCommand {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	pending := []DeleteCommand{}
	for _, command := range ToDelete[host] {
		if slices.Contains(done, command.ID) {
			continue
		}
		if err, ok := failed[command.ID]; ok {
			command.failures++
			if command.failures >= maxDeleteAttempts {
				fmt.Printf("ERROR: gave up deleting docker logs of %s on %s: %q\n", command.Container, host, err)
				continue
			}
		}
		pending = append(pending, command)
	}
	if len(pending) == 0 {
		delete(ToDelete, host)
	} else {
		ToDelete[host] = pending
	}
	return slices.Clone(pending)
}

// CheckDatabases reports the package-level LevelDB handles that failed to open.
// They are opened in variable initialisers, so without this the first
// dereference is a nil-pointer panic on whichever goroutine gets there first.
func CheckDatabases() error {
	failures := []string{}
	for name, err := range map[string]error{
		"leveldb/hostaliases":   AliasDBErr,
		"leveldb/favourites":    FavsDBErr,
		"leveldb/groups":        GroupsDBErr,
		"leveldb/users":         UsersDBErr,
		"leveldb/tokens":        TokensDBErr,
		"leveldb/usersSettings": SettingsDBErr,
		"leveldb/sessions":      SessionsDBErr,
	} {
		if err != nil {
			failures = append(failures, name+": "+err.Error())
		}
	}
	if len(failures) == 0 {
		return nil
	}
	sort.Strings(failures)
	return errors.New("unable to open " + strings.Join(failures, "; "))
}
