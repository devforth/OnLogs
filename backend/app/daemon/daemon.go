package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	db "github.com/devforth/OnLogs/app/db"
	"github.com/tv42/httpunix"
)

// creates stream that writes logs from every docker container to leveldb
func CreateLogsStream(containerName string) {
	unix := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	unix.RegisterLocation("daemon", "/var/run/docker.sock")

	var client = http.Client{Transport: unix}
	resp, _ := client.Get(
		"http+unix://daemon/containers/" + containerName + "/logs?stdout=true&stderr=true&timestamps=true&follow=true",
	)
	reader := bufio.NewReader(resp.Body)
	lastSleep := time.Now().Unix()
	for {
		line, _ := reader.ReadBytes('\n')
		logLine := string(line)
		if strings.Compare("", logLine) == 0 {
			continue
		}

		logItem := &db.LogItem{
			Datetime: logLine[:30],
			Message:  logLine[31 : len(logLine)-2],
		}
		db.StoreItem(containerName, logItem)

		if time.Now().Unix()-lastSleep > 5 {
			go time.Sleep(5 * time.Second)
			lastSleep = time.Now().Unix()
		}
	}
}

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
