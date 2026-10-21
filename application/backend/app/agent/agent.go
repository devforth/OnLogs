package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

func post(path string, body map[string]any) (*http.Response, error) {
	payload, _ := json.Marshal(body)
	return http.Post(os.Getenv("HOST")+path, "application/json", bytes.NewBuffer(payload))
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

func sendLog(token, container, timestamp, message, eventID string) bool {
	resp, err := post("/api/v1/addLogLine", map[string]any{
		"Host":      util.GetHost(),
		"Token":     token,
		"LogLine":   []string{timestamp, message},
		"Container": container,
		"EventID":   eventID,
	})
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func SendLogMessage(token string, container string, message_item []string) bool {
	eventID := containerdb.NewEventID()
	if sendLog(token, container, message_item[0], message_item[1], eventID) {
		return true
	}
	if buffer := util.GetDB(util.GetHost(), container, "brokenlogs"); buffer != nil {
		buffer.Put([]byte(message_item[0]+" +"+eventID), []byte(message_item[1]), nil)
	}
	return false
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

// Returns false when the master is still unreachable, so the caller stops.
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
		if !sendLog(token, container, timestamp, string(iter.Value()), eventID) {
			return false
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

func AskForDelete() {
	resp, err := post("/api/v1/askForDelete", identity())
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "" {
		var toDelete map[string][]string
		json.Unmarshal(body, &toDelete)

		for _, container := range toDelete["Services"] {
			util.DeleteDockerLogs(util.GetHost(), container)
		}
	}
}
