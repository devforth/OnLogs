package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/devforth/OnLogs/app/util"
)

func SendInitRequest(containers []string) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Hostname": util.GetHost(),
		"Token":    os.Getenv("ONLOGS_TOKEN"),
		"Services": containers,
	})
	responseBody := bytes.NewBuffer(postBody)

	resp, err := http.Post(os.Getenv("HOST")+"/api/v1/addHost", "application/json", responseBody)
	if err != nil {
		panic("ERROR: Can't send request to host: " + err.Error())
	}

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		panic("ERROR: Response status from host is " + resp.Status + "\nResponse body: " + string(b))
	}
}

func SendLogMessage(token string, container string, message_item []string) bool {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Host":      util.GetHost(),
		"Token":     token,
		"LogLine":   []string{message_item[0], message_item[1]},
		"Container": container,
	})
	resp, err := http.Post(os.Getenv("HOST")+"/api/v1/addLogLine", "application/json", bytes.NewBuffer(postBody))
	if err == nil {
		defer resp.Body.Close()
	}
	if err != nil || resp.StatusCode != 200 {
		if buffer := util.GetDB(util.GetHost(), container, "brokenlogs"); buffer != nil {
			buffer.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
		}
		return false
	}
	return true
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
		value := append([]byte{}, iter.Value()...)
		if !SendLogMessage(token, container, []string{string(key), string(value)}) {
			return false
		}
		buffer.Delete(key, nil)
	}
	return true
}

func SendUpdate(containers []string) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Hostname": util.GetHost(),
		"Token":    os.Getenv("ONLOGS_TOKEN"),
		"Services": containers,
	})
	responseBody := bytes.NewBuffer(postBody)

	http.Post(os.Getenv("HOST")+"/api/v1/addHost", "application/json", responseBody)
	AskForDelete()
}

func AskForDelete() {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Hostname": util.GetHost(),
		"Token":    os.Getenv("ONLOGS_TOKEN"),
	})
	responseBody := bytes.NewBuffer(postBody)

	resp, err := http.Post(os.Getenv("HOST")+"/api/v1/askForDelete", "application/json", responseBody)
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
