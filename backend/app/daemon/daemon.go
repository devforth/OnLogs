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

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tv42/httpunix"
)

func createLogMessage(db *leveldb.DB, message string) {
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), " +")[0], " ", "T", 1)
	if len(datetime) < 29 {
		datetime = datetime + strings.Repeat("0", 29-len(datetime))
	}
	db.Put([]byte(datetime+"Z"), []byte(message), nil)
}

func putLogMessage(db *leveldb.DB, message string) {
	db.Put([]byte(message[:30]), []byte(message[31:len(message)-1]), nil)
}

// creates stream that writes logs from every docker container to leveldb
func CreateDaemonToDBStream(containerName string) {
	db := vars.ActiveDBs[containerName]
	unix := &httpunix.Transport{
		DialTimeout:           100 * time.Millisecond,
		RequestTimeout:        1 * time.Second,
		ResponseHeaderTimeout: 1 * time.Second,
	}
	unix.RegisterLocation("daemon", "/var/run/docker.sock")
	var client = http.Client{Transport: unix}
	resp, _ := client.Get(
		"http+unix://daemon/containers/" + containerName + "/logs?stdout=true&stderr=true&timestamps=true&follow=true&since=" + strconv.FormatInt(time.Now().Unix(), 10),
	)
	createLogMessage(db, "ONLOGS: Container listening started!")

	reader := bufio.NewReader(resp.Body)
	lastSleep := time.Now().Unix()
	defer db.Close()
	for {
		logLine, get_string_error := reader.ReadString('\n')
		if get_string_error != nil {
			newDaemonStreams := []string{}
			for _, stream := range vars.Active_Daemon_Streams {
				if stream != containerName {
					newDaemonStreams = append(newDaemonStreams, stream)
				}
			}
			vars.Active_Daemon_Streams = newDaemonStreams
			createLogMessage(db, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")")
			return
		}

		to_put := []byte(logLine)
		if []byte(logLine)[0] < 32 { // is it ok?
			to_put = to_put[8:]
		}
		putLogMessage(db, string(to_put))
		to_send, _ := json.Marshal([]string{string(to_put[:30]), string(to_put[31 : len(to_put)-1])})
		for _, c := range vars.Connections[containerName] {
			c.WriteMessage(1, to_send)
		}

		if time.Now().Unix()-lastSleep > 1 {
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
