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

func createLogMessage(db *leveldb.DB, message string) string {
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), " +")[0], " ", "T", 1)
	if len(datetime) < 29 {
		datetime = datetime + strings.Repeat("0", 29-len(datetime))
	}
	if db != nil {
		db.Put([]byte(datetime+"Z"), []byte(message), nil)
	}
	return datetime + "Z " + message
}

func putLogMessage(db *leveldb.DB, message string) {
	if len(message) < 31 {
		return
	}

	message_text := message[31:]

	if strings.Contains(message_text, "ERROR") || strings.Contains(message_text, "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_text, "Error") || strings.Contains(message_text, "Err") {
		vars.Counters_For_Last_30_Min["error"]++
	} else if strings.Contains(message_text, "WARN") || strings.Contains(message_text, "WARNING") { // const statuses_warnings = ["WARN", "WARNING"];
		vars.Counters_For_Last_30_Min["warning"]++
	} else if strings.Contains(message_text, "DEBUG") { // const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
		vars.Counters_For_Last_30_Min["debug"]++
	} else if strings.Contains(message_text, "INFO") {
		vars.Counters_For_Last_30_Min["info"]++
	}

	db.Put([]byte(message[:30]), []byte(message_text), nil)
}

func sendLogMessage(container string, message string) {
	postBody, _ := json.Marshal(map[string]interface{}{
		"Host":      util.GetHost(),
		"LogLine":   []string{message[:30], message[31:]},
		"Container": container,
	})
	http.Post(os.Getenv("HOST")+"/api/v1/addLogLine", "application/json", bytes.NewBuffer(postBody))
	// if err != nil {
	// 	fmt.Println("ERROR: Can't send request to host!\n" + err.Error())
	// 	fmt.Println("WARN: Message is not sent: " + message)
	// }

	// if resp.StatusCode != 200 {
	// 	fmt.Println("ERROR: Response status from host is " + resp.Status) // TODO: Improve text with host response body
	// 	fmt.Println("WARN: Message is not sent: " + message)
	// }
}

func validateMessage(message string) ([]byte, bool) {
	to_put := []byte(message)[:len(message)]
	if len(to_put) < 31 {
		return nil, false
	}

	if []byte(message)[0] < 32 || []byte(message)[0] > 126 { // is it ok?
		to_put = to_put[8:]
	}

	// if []byte(message)[0] != 50 { // is it ok?
	// 	to_put = to_put[6:]
	// }

	return to_put, true
}

func createConnection(containerName string) net.Conn {
	connection, _ := net.Dial("unix", "/var/run/docker.sock")
	fmt.Fprintf(
		connection,
		"GET /containers/"+containerName+"/logs?stdout=true&stderr=true&timestamps=true&follow=true&since="+strconv.FormatInt(time.Now().Unix(), 10)+" HTTP/1.0\r\n\r\n",
	)
	return connection
}

func readHeader(reader bufio.Reader) {
	for { // reading resp header
		tmp, _ := reader.ReadString('\n')
		if tmp[:len(tmp)-2] == "" {
			reader.ReadString('\n')
			break
		}
	}
}

func closeActiveStream(containerName string) {
	newDaemonStreams := []string{}
	for _, stream := range vars.Active_Daemon_Streams {
		if stream != containerName {
			newDaemonStreams = append(newDaemonStreams, stream)
		}
	}
	vars.Active_Daemon_Streams = newDaemonStreams
}

func CreateDaemonToHostStream(containerName string) {
	connection := createConnection(containerName)
	reader := bufio.NewReader(connection)
	readHeader(*reader)

	sendLogMessage(containerName, createLogMessage(nil, "ONLOGS: Container listening started!"))

	lastSleep := time.Now().Unix()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n') // TODO read bytes instead of strings
		if get_string_error != nil {
			closeActiveStream(containerName)
			sendLogMessage(containerName, createLogMessage(nil, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")"))
			return
		}

		to_put, valid := validateMessage(logLine)
		if !valid {
			continue
		}

		sendLogMessage(containerName, string(to_put))

		if time.Now().Unix()-lastSleep > 1 {
			time.Sleep(5 * time.Millisecond)
			lastSleep = time.Now().Unix()
		}
	}
}

// creates stream that writes logs from every docker container to leveldb
func CreateDaemonToDBStream(containerName string) {
	connection := createConnection(containerName)
	reader := bufio.NewReader(connection)
	readHeader(*reader)

	db := vars.ActiveDBs[containerName]
	// defer db.Close()
	createLogMessage(db, "ONLOGS: Container listening started!")

	lastSleep := time.Now().Unix()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n')
		if get_string_error != nil {
			closeActiveStream(containerName)
			createLogMessage(db, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")")
			return
		}

		to_put, valid := validateMessage(logLine)
		if !valid {
			continue
		}

		putLogMessage(db, string(to_put))

		to_send, _ := json.Marshal([]string{string(to_put[:30]), string(to_put[31:])})
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
