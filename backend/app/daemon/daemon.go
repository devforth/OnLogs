package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nsqio/go-diskqueue"
	"github.com/tv42/httpunix"
)

// creates stream that writes logs from every docker container to leveldb
func CreateDaemonToLogfileStream(containerName string, dq diskqueue.Interface) {
	unix := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	unix.RegisterLocation("daemon", "/var/run/docker.sock")

	var client = http.Client{Transport: unix}
	curTime := strconv.FormatInt(time.Now().Unix(), 10)
	resp, _ := client.Get(
		"http+unix://daemon/containers/" + containerName + "/logs?stdout=true&stderr=true&timestamps=true&follow=true&since=" + curTime,
	)
	reader := bufio.NewReader(resp.Body)
	lastSleep := time.Now().Unix()
	var counter uint64 = 0
	for {
		logLine, _ := reader.ReadString('\n')
		if strings.Compare("", logLine) == 0 {
			continue
		}

		var err error
		if []byte(logLine)[0] == 1 || []byte(logLine)[0] == 2 { // is it ok?
			err = dq.Put([]byte(logLine)[8:])
		} else {
			err = dq.Put([]byte(logLine))
		}

		if err != nil {
			time.Sleep(1 * time.Millisecond)
			continue
		}
		counter += 1
		if time.Now().Unix()-lastSleep > 5 {
			time.Sleep(5 * time.Millisecond)
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
