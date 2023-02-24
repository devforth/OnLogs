package agent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
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
		b, _ := ioutil.ReadAll(resp.Body)
		panic("ERROR: Response status from host is " + resp.Status + "\nResponse body: " + string(b))
	}
}

func SendLogMessage(container string, message_item []string) bool {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Host":      util.GetHost(),
		"LogLine":   []string{message_item[0], message_item[1]},
		"Container": container,
	})
	resp, err := http.Post(os.Getenv("HOST")+"/api/v1/addLogLine", "application/json", bytes.NewBuffer(postBody))
	if err != nil || resp.StatusCode != 200 {
		vars.BrokenLogs_DBs[container].Put([]byte(message_item[0]), []byte(message_item[1]), nil)
		return false
	}
	return true
}

func TryResend() {
	containers, _ := os.ReadDir("leveldb/hosts/" + util.GetHost() + "/containers/")
	for _, container := range containers {
		tmpDB := vars.BrokenLogs_DBs[container.Name()]
		if tmpDB == nil {
			tmpDB = util.GetDB(util.GetHost(), container.Name(), "/brokenLogs")
			defer tmpDB.Close()
		}

		iter := tmpDB.NewIterator(nil, nil)
		defer iter.Release()
		iter.First()
		if iter.Value() == nil {
			continue
		}

		if !SendLogMessage(container.Name(), []string{string(iter.Key()), string(iter.Value())}) {
			return
		}
		tmpDB.Delete(iter.Key(), nil)

		for iter.Next() {
			if !SendLogMessage(container.Name(), []string{string(iter.Key()), string(iter.Value())}) {
				return
			}
			tmpDB.Delete(iter.Key(), nil)
		}
	}
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

	resp, _ := http.Post(os.Getenv("HOST")+"/api/v1/askForDelete", "application/json", responseBody)

	body, _ := ioutil.ReadAll(resp.Body)
	if string(body) != "" {
		var toDelete map[string][]string
		json.Unmarshal(body, &toDelete)

		for _, container := range toDelete["Services"] {
			util.DeleteDockerLogs(util.GetHost(), container)
		}
	}
}
