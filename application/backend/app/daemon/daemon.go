package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/api/types/container"
	"github.com/syndtr/goleveldb/leveldb"
)

type DaemonService struct {
	DockerClient *docker.DockerService
}

func createLogMessage(db *leveldb.DB, host string, container string, message string) string {
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), " +")[0], " ", "T", 1)
	if len(datetime) < 29 {
		datetime = datetime + strings.Repeat("0", 29-len(datetime))
	}
	if db != nil {
		containerdb.PutLogMessage(db, host, container, []string{datetime + "Z", message})
	}
	return datetime + "Z " + message
}

func validateMessage(message string) (string, bool) {
	for !strings.HasPrefix(message, vars.Year) {
		message = message[1:]
		if len(message) < 31 {
			return "", false
		}
	}

	return message, true
}

func closeActiveStream(containerName string) {
	newDaemonStreams := []string{}
	for _, stream := range vars.Active_Daemon_Streams {
		if stream != containerName {
			newDaemonStreams = append(newDaemonStreams, stream)
		}
	}
	if vars.ActiveDBs[containerName] != nil {
		vars.ActiveDBs[containerName].Close()
	}
	vars.ActiveDBs[containerName] = nil
	vars.Active_Daemon_Streams = newDaemonStreams
}

func (h *DaemonService) CreateDaemonToHostStream(ctx context.Context, containerName string) {
	rc, err := h.DockerClient.Client.ContainerLogs(ctx, containerName, container.LogsOptions{ShowStdout: true, ShowStderr: true, Timestamps: true, Follow: true, Since: strconv.FormatInt(time.Now().Add(-5*time.Second).Unix(), 10)})
	if err != nil {
		closeActiveStream(containerName)
		return
	}
	defer rc.Close()

	reader := bufio.NewReader(rc)

	host := util.GetHost()
	token := os.Getenv("ONLOGS_TOKEN")
	agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening started!"), " ", 2))

	lastSleep := time.Now().Unix()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n')
		if get_string_error != nil {
			if get_string_error == io.EOF {
				closeActiveStream(containerName)
				agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening stopped! (EOF)"), " ", 2))
				return
			}
			closeActiveStream(containerName)
			agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")"), " ", 2))
			return
		}

		logLine, valid := validateMessage(logLine)
		if !valid {
			continue
		}
		message_item := strings.SplitN(logLine, " ", 2)
		agent.SendLogMessage(token, containerName, message_item)

		if time.Now().Unix()-lastSleep > 1 {
			time.Sleep(5 * time.Millisecond)
			lastSleep = time.Now().Unix()
		}
	}
}

// creates stream that writes logs from every docker container to leveldb
func (h *DaemonService) CreateDaemonToDBStream(ctx context.Context, containerName string) {
	rc, err := h.DockerClient.Client.ContainerLogs(ctx, containerName, container.LogsOptions{ShowStdout: true, ShowStderr: true, Timestamps: true, Follow: true, Since: strconv.FormatInt(time.Now().Add(-5*time.Second).Unix(), 10)})
	if err != nil {
		closeActiveStream(containerName)
		return
	}
	defer rc.Close()

	reader := bufio.NewReader(rc)

	host := util.GetHost()
	current_db := util.GetDB(host, containerName, "logs")
	createLogMessage(current_db, host, containerName, "ONLOGS: Container listening started!")

	defer current_db.Close()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n')
		if get_string_error != nil {
			if get_string_error == io.EOF {
				closeActiveStream(containerName)
				createLogMessage(current_db, host, containerName, "ONLOGS: Container listening stopped! (EOF)")
				return
			}
			closeActiveStream(containerName)
			createLogMessage(current_db, host, containerName, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")")
			return
		}

		logLine, valid := validateMessage(logLine)
		if !valid {
			continue
		}
		logItem := strings.SplitN(logLine, " ", 2)

		err := containerdb.PutLogMessage(current_db, host, containerName, logItem)
		if err != nil {
			if err.Error() == "leveldb: closed" {
				current_db = vars.ActiveDBs[containerName]
				containerdb.PutLogMessage(current_db, host, containerName, logItem)
			} else {
				fmt.Println("ERROR: " + err.Error())
				closeActiveStream(containerName)
				return
			}
		}
		to_send, _ := json.Marshal(logItem)
		for _, c := range vars.Connections[containerName] {
			c.WriteMessage(1, to_send)
		}

		time.Sleep(70 * time.Microsecond)
	}
}

// returns list of names of docker containers from docker daemon
func (h *DaemonService) GetContainersList(ctx context.Context) []string {
	result, err := h.DockerClient.GetContainerNames(ctx)
	if err != nil {
		fmt.Println("ERROR: failed to get containers list from docker daemon:", err)
		return vars.DockerContainers
	}

	var names []string

	containersMetaDB := vars.ContainersMeta_DBs[util.GetHost()]
	if containersMetaDB == nil {
		containersMetaDB, err := leveldb.OpenFile("leveldb/hosts/"+util.GetHost()+"/containersMeta", nil)
		if err != nil {
			panic(err)
		}
		vars.ContainersMeta_DBs[util.GetHost()] = containersMetaDB
	}
	containersMetaDB = vars.ContainersMeta_DBs[util.GetHost()]

	for i := range result {
		name := result[i].Name
		id := result[i].ID

		names = append(names, name)
		containersMetaDB.Put([]byte(name), []byte(id), nil)
	}

	return names
}

func (h *DaemonService) GetContainerImageNameByContainerID(ctx context.Context, containerID string) string {
	result, err := h.DockerClient.GetContainerImageNameByContainerID(ctx, containerID)
	if err != nil {
		return ""
	}

	return result
}
