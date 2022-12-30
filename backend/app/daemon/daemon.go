package daemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
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

func sendLogMessage(container string, message string) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Host":      util.GetHost(),
		"LogLine":   []string{message[:30], message[31 : len(message)-1]},
		"Container": container,
	})
	responseBody := bytes.NewBuffer(postBody)

	http.Post(os.Getenv("HOST")+"/api/v1/addLogLine", "application/json", responseBody)
	// if err != nil {
	// 	fmt.Println("ERROR: Can't send request to host!\n" + err.Error())
	// 	fmt.Println("WARN: Message is not sent: " + message)
	// }

	// if resp.StatusCode != 200 {
	// 	fmt.Println("ERROR: Response status from host is " + resp.Status) // TODO: Improve text with host response body
	// 	fmt.Println("WARN: Message is not sent: " + message)
	// }
}

// creates stream that writes logs from every docker container to leveldb
func CreateDaemonToDBStream(containerName string) {
	db := vars.ActiveDBs[containerName]
	connection, _ := net.Dial("unix", "/var/run/docker.sock")
	fmt.Fprintf(
		connection,
		"GET /containers/"+containerName+"/logs?stdout=true&stderr=true&timestamps=true&follow=true&since="+strconv.FormatInt(time.Now().Unix(), 10)+" HTTP/1.0\r\n\r\n",
	)

	createLogMessage(db, "ONLOGS: Container listening started!")

	reader := bufio.NewReader(connection)
	lastSleep := time.Now().Unix()
	defer db.Close()

	for { // reading resp header
		tmp, _ := reader.ReadString('\n')
		if tmp[:len(tmp)-2] == "" {
			tmp, _ = reader.ReadString('\n')
			break
		}
	}

	for { // reading body
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
		if len(to_put) < 31 {
			continue
		}

		if []byte(logLine)[0] < 32 || []byte(logLine)[0] > 126 { // is it ok?
			to_put = to_put[8:]
		}

		if os.Getenv("CLIENT") != "" {
			sendLogMessage(containerName, string(to_put))
		} else {
			putLogMessage(db, string(to_put))

			to_send, _ := json.Marshal([]string{string(to_put[:30]), string(to_put[31 : len(to_put)-1])})
			for _, c := range vars.Connections[containerName] {
				c.WriteMessage(1, to_send)
			}
		}

		if time.Now().Unix()-lastSleep > 1 {
			time.Sleep(5 * time.Millisecond)
			lastSleep = time.Now().Unix()
		}
	}
}

// make request to docker socket
func makeSocketRequest(path string) []byte {
	connection, _ := net.Dial("unix", "/var/run/docker.sock")
	fmt.Fprintf(connection, "GET /"+path+" HTTP/1.0\r\n\r\n")

	body, _ := ioutil.ReadAll(connection)

	connection.Close()
	return body
}

// returns list of names of docker containers from docker daemon
func GetContainersList() []string {
	var result []map[string]any

	body := string(makeSocketRequest("containers/json"))
	body = strings.Split(body, "\r\n\r\n")[1]
	json.Unmarshal([]byte(body), &result)

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
