package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

// Above the 30s dial timeout, so an unreachable master still fails as a dial error.
var client = &http.Client{Timeout: time.Minute}

func post(path string, body map[string]any) (*http.Response, error) {
	payload, _ := json.Marshal(body)
	return client.Post(os.Getenv("HOST")+path, "application/json", bytes.NewBuffer(payload))
}

func identity() map[string]any {
	return map[string]any{
		"Hostname": util.GetHost(),
		"Token":    os.Getenv("ONLOGS_TOKEN"),
	}
}

func SendInitRequest(containers []string) {
	body := identity()
	body["Services"] = containers

	resp, err := post("/api/v1/addHost", body)
	if err != nil {
		panic("ERROR: Can't send request to host: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		panic("ERROR: Response status from host is " + resp.Status + "\nResponse body: " + string(b))
	}
}

func sendLog(token, container, timestamp, message, eventID string) error {
	resp, err := post("/api/v1/addLogLine", map[string]any{
		"Host":      util.GetHost(),
		"Token":     token,
		"LogLine":   []string{timestamp, message},
		"Container": container,
		"EventID":   eventID,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

func SendLogMessage(token string, container string, message_item []string) (bool, error) {
	eventID := containerdb.NewEventID()
	if sendLog(token, container, message_item[0], message_item[1], eventID) == nil {
		return true, nil
	}
	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	if buffer == nil {
		return false, fmt.Errorf("no pending queue for %s", container)
	}
	return false, buffer.Put([]byte(message_item[0]+" +"+eventID), []byte(message_item[1]), nil)
}

func TryResend() {
	token := os.Getenv("ONLOGS_TOKEN")
	containers, _ := os.ReadDir("leveldb/hosts/" + util.GetHost() + "/containers/")
	for _, container := range containers {
		if !resendContainerBuffer(token, container.Name()) {
			return
		}
	}
}

func resendContainerBuffer(token string, container string) bool {
	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	if buffer == nil {
		return true
	}

	iter := buffer.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := append([]byte{}, iter.Key()...)
		// Entries queued by older agents are keyed by the bare timestamp.
		timestamp, eventID, _ := strings.Cut(string(key), " +")
		if err := sendLog(token, container, timestamp, string(iter.Value()), eventID); err != nil {
			var opErr *net.OpError
			return !errors.As(err, &opErr) || opErr.Op != "dial"
		}
		buffer.Delete(key, nil)
	}
	return true
}

func SendUpdate(containers []string) {
	body := identity()
	body["Services"] = containers

	if resp, err := post("/api/v1/addHost", body); err == nil {
		resp.Body.Close()
	}
	AskForDelete()
}

var (
	deletesDone   []string
	deletesFailed = map[string]string{}
)

type deleteReply struct {
	Services []string
	Commands []struct{ ID, Container string }
}

func pollDeletes() (deleteReply, bool) {
	body := identity()
	body["Done"] = append([]string{}, deletesDone...)
	body["Failed"] = deletesFailed

	var reply deleteReply
	resp, err := post("/api/v1/askForDelete", body)
	if err != nil {
		return reply, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&reply) != nil {
		return reply, false
	}
	deletesDone, deletesFailed = nil, map[string]string{}
	return reply, true
}

func deleteDockerLogs(container string) error {
	err := util.DeleteDockerLogs(util.GetHost(), container)
	if err != nil {
		fmt.Println("ERROR: unable to delete docker logs of "+container+":", err)
	}
	return err
}

func AskForDelete() {
	tried := map[string]bool{}
	for {
		reply, ok := pollDeletes()
		if !ok {
			return
		}
		if reply.Commands == nil {
			// An older master sends bare names and has already forgotten them.
			for _, container := range reply.Services {
				deleteDockerLogs(container)
			}
			return
		}

		fresh := false
		for _, command := range reply.Commands {
			if tried[command.ID] {
				continue
			}
			tried[command.ID], fresh = true, true
			if err := deleteDockerLogs(command.Container); err != nil {
				deletesFailed[command.ID] = err.Error()
			} else {
				deletesDone = append(deletesDone, command.ID)
			}
		}
		if !fresh {
			return
		}
	}
}
