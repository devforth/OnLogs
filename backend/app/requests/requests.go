package requests

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/tv42/httpunix"
)

// make request to docker socket
func makeSocketRequest(path string) []byte {
	unix := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	unix.RegisterLocation("daemon", "/var/run/docker.sock")

	var client = http.Client{
		Transport: unix,
	}

	resp, _ := client.Get("http+unix://daemon/" + path)
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	return body
}

// returns list of names of docker containers from docker daemon
func GetContainersList() []string {
	body := makeSocketRequest("containers/json")
	var result []map[string]any
	json.Unmarshal(body, &result)

	var names []string
	for i := 0; i < len(result); i++ {
		for key, element := range result[i] {
			if key == "Names" {
				name := element.([]interface{})[0].(string)
				str := fmt.Sprintf("%v", name)
				names = append(names, string(str[1:]))
			}
		}
	}
	return names
}

// returns log of a container
func GetContainerLogs(containerName string, limit int, offset int) string {
	body := makeSocketRequest("containers/" + containerName + "/logs?stdout=true&stderr=true&timestamps=true")
	return string(body)
}