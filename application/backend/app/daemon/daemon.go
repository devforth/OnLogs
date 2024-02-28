package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func createLogMessage(db *leveldb.DB, host string, container string, message string) string {
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), " +")[0], " ", "T", 1)
	if len(datetime) < 29 {
		datetime = datetime + strings.Repeat("0", 29-len(datetime))
	}
	if db != nil {
		containerdb.PutLogMessage(db, host, container, []string{datetime + "Z", message})
	}
	return datetime + "Z " + message
}

func validateMessage(message string) (string, bool) {
	for !strings.HasPrefix(message, vars.Year) {
		message = message[1:]
		if len(message) < 31 {
			return "", false
		}
	}

	return message, true
}

func createConnection(containerName string) net.Conn {
	connection, _ := net.Dial("unix", os.Getenv("DOCKER_SOCKET_PATH"))
	fmt.Fprintf(
		connection,
		"GET /containers/"+containerName+"/logs?stdout=true&stderr=true&timestamps=true&follow=true&since="+strconv.FormatInt(time.Now().Add(-5*time.Second).Unix(), 10)+" HTTP/1.0\r\n\r\n",
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
	if vars.ActiveDBs[containerName] != nil {
		vars.ActiveDBs[containerName].Close()
	}
	vars.ActiveDBs[containerName] = nil
	vars.Active_Daemon_Streams = newDaemonStreams
}

func CreateDaemonToHostStream(containerName string) {
	connection := createConnection(containerName)
	reader := bufio.NewReader(connection)
	readHeader(*reader)

	host := util.GetHost()
	token := os.Getenv("ONLOGS_TOKEN")
	agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening started!"), " ", 2))

	lastSleep := time.Now().Unix()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n') // TODO read bytes instead of strings
		if get_string_error != nil {
			closeActiveStream(containerName)
			agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")"), " ", 2))
			return
		}

		logLine, valid := validateMessage(logLine)
		if !valid {
			continue
		}
		message_item := strings.SplitN(logLine, " ", 2)
		agent.SendLogMessage(token, containerName, message_item)

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

	host := util.GetHost()
	current_db := util.GetDB(host, containerName, "logs")
	createLogMessage(current_db, host, containerName, "ONLOGS: Container listening started!")

	defer current_db.Close()
	for { // reading body
		logLine, get_string_error := reader.ReadString('\n')
		if get_string_error != nil {
			closeActiveStream(containerName)
			createLogMessage(current_db, host, containerName, "ONLOGS: Container listening stopped! ("+get_string_error.Error()+")")
			return
		}

		logLine, valid := validateMessage(logLine)
		if !valid {
			continue
		}
		logItem := strings.SplitN(logLine, " ", 2)

		err := containerdb.PutLogMessage(current_db, host, containerName, logItem)
		if err != nil {
			if err.Error() == "leveldb: closed" {
				current_db = vars.ActiveDBs[containerName]
				containerdb.PutLogMessage(current_db, host, containerName, logItem)
			} else {
				fmt.Println("ERROR: " + err.Error())
				closeActiveStream(containerName)
				return
			}
		}
		to_send, _ := json.Marshal(logItem)
		for _, c := range vars.Connections[containerName] {
			c.WriteMessage(1, to_send)
		}

		time.Sleep(70 * time.Microsecond)
	}
}

// make request to docker socket
func makeSocketRequest(path string) []byte {
	connection, err := net.Dial("unix", os.Getenv("DOCKER_SOCKET_PATH"))
	if err != nil {
		panic(err)
	}
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

	containersMetaDB := vars.ContainersMeta_DBs[util.GetHost()]
	if containersMetaDB == nil {
		containersMetaDB, err := leveldb.OpenFile("leveldb/hosts/"+util.GetHost()+"/containersMeta", nil)
		if err != nil {
			panic(err)
		}
		vars.ContainersMeta_DBs[util.GetHost()] = containersMetaDB
	}
	containersMetaDB = vars.ContainersMeta_DBs[util.GetHost()]

	for i := 0; i < len(result); i++ {
		name := fmt.Sprintf("%v", result[i]["Names"].([]interface{})[0].(string))[1:]
		id := result[i]["Id"].(string)

		names = append(names, name)
		containersMetaDB.Put([]byte(name), []byte(id), nil)
	}

	return names
}
