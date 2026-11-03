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
	"sync"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

// Above the 30s dial timeout, so an unreachable master still fails as a dial error.
var client = &http.Client{Timeout: time.Minute}

func post(path string, body map[string]any) (*http.Response, error) {
	// HTML escaping turns each < > & into six bytes and can push a line past
	// the master's 1 MiB body limit.
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(body)
	return client.Post(os.Getenv("HOST")+path, "application/json", &payload)
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
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusRequestEntityTooLarge {
		return fmt.Errorf("%w: %s", errRejected, resp.Status)
	}
	if resp.StatusCode != 200 {
		return errors.New(resp.Status)
	}
	return nil
}

// The master answers this line the same way on every retry.
var errRejected = errors.New("rejected by the master")

var (
	// Known queue state per container. The disk is read once, for a queue left
	// by an earlier run: iterating a drained queue walks all its tombstones.
	queued sync.Map
	// One drain per queue at a time: lines queued by older agents carry no
	// event id, so the master stores a double send twice.
	queueLocks sync.Map
)

func lockQueue(container string) func() {
	lock, _ := queueLocks.LoadOrStore(container, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func hasQueued(container string) bool {
	if known, ok := queued.Load(container); ok {
		return known.(bool)
	}
	has := false
	if _, err := os.Stat("leveldb/hosts/" + util.GetHost() + "/containers/" + container + "/brokenlogs"); err == nil {
		if buffer := util.GetDB(util.GetHost(), container, "brokenlogs"); buffer != nil {
			iter := buffer.NewIterator(nil, nil)
			has = iter.First()
			iter.Release()
		}
	}
	queued.Store(container, has)
	return has
}

func SendLogMessage(token string, container string, message_item []string) (bool, error) {
	defer lockQueue(container)()
	eventID := containerdb.NewEventID()
	// Live viewers see lines in arrival order, so queued ones go first.
	if hasQueued(container) {
		resendContainerBuffer(token, container)
	}
	if !hasQueued(container) {
		err := sendLog(token, container, message_item[0], message_item[1], eventID)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, errRejected) {
			fmt.Println("ERROR: the master rejected a line of "+container+", dropping it:", err)
			return false, err
		}
	}
	buffer := util.GetDB(util.GetHost(), container, "brokenlogs")
	if buffer == nil {
		return false, fmt.Errorf("no pending queue for %s", container)
	}
	if err := buffer.Put([]byte(message_item[0]+" +"+eventID), []byte(message_item[1]), nil); err != nil {
		return false, err
	}
	queued.Store(container, true)
	return false, nil
}

func TryResend() {
	token := os.Getenv("ONLOGS_TOKEN")
	containers, _ := os.ReadDir("leveldb/hosts/" + util.GetHost() + "/containers/")
	for _, container := range containers {
		unlock := lockQueue(container.Name())
		ok := resendContainerBuffer(token, container.Name())
		unlock()
		if !ok {
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
			if !errors.Is(err, errRejected) {
				var opErr *net.OpError
				return !errors.As(err, &opErr) || opErr.Op != "dial"
			}
			fmt.Println("ERROR: the master rejected a queued line of "+container+", dropping it:", err)
		}
		buffer.Delete(key, nil)
	}
	if iter.Error() == nil {
		queued.Store(container, false)
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
