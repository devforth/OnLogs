package agent

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
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
		b, _ := ioutil.ReadAll(resp.Body)
		panic("ERROR: Response status from host is " + resp.Status + "\nResponse body: " + string(b))
	}
}

// TODO handle unsended logs better
func SendLogMessage(container string, message_item []string) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Host":      util.GetHost(),
		"LogLine":   []string{message_item[0], message_item[1]},
		"Container": container,
	})
	http.Post(os.Getenv("HOST")+"/api/v1/addLogLine", "application/json", bytes.NewBuffer(postBody))
	// if resp.StatusCode != 200 {
	// 	time.Sleep(1 * time.Minute)
	// 	SendLogMessage(container, message)
	// }
}

func SendUpdate(containers []string) {

}
