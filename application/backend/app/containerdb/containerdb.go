package containerdb

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func containStr(a string, b string, caseSens bool) bool {
	if caseSens {
		return strings.Contains(a, b)
	}

	return strings.Contains(strings.ToLower(a), strings.ToLower(b))
}

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) {
	if len(message_item[0]) < 30 {
		fmt.Println("ERROR: got broken timestamp: ", message_item)
		return
	}

	for !strings.HasPrefix(message_item[0], "2") && len(message_item[0]) > 20 {
		message_item[0] = message_item[0][1:]
	}

	if host == "" {
		panic("Host is not mentioned!")
	}
	location := host + "/" + container

	if strings.Contains(message_item[1], "ERROR") || strings.Contains(message_item[1], "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_item[1], "Error") || strings.Contains(message_item[1], "Err") {
		vars.Counters_For_Containers_Last_30_Min[location]["error"]++

	} else if strings.Contains(message_item[1], "WARN") || strings.Contains(message_item[1], "WARNING") { // const statuses_warnings = ["WARN", "WARNING"];
		vars.Counters_For_Containers_Last_30_Min[location]["warn"]++

	} else if strings.Contains(message_item[1], "DEBUG") { // const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
		vars.Counters_For_Containers_Last_30_Min[location]["debug"]++

	} else if strings.Contains(message_item[1], "INFO") {
		vars.Counters_For_Containers_Last_30_Min[location]["info"]++

	} else {
		vars.Counters_For_Containers_Last_30_Min[location]["other"]++
	}

	db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
}

func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool) [][]string {
	var db *leveldb.DB
	var err error
	db = vars.ActiveDBs[container]
	if db == nil {
		path := "leveldb/hosts/" + host + "/containers/" + container + "/logs"

		_, pathErr := os.Stat(path)
		if os.IsNotExist(pathErr) {
			return [][]string{}
		}

		db, err = leveldb.OpenFile(path, nil)
		if err != nil {
			db, _ = leveldb.RecoverFile(path, nil)
		}
		defer db.Close()
	}

	iter := db.NewIterator(nil, nil)
	counter := 0
	to_return := [][]string{}

	iter.Last()
	if startWith != "" {
		if !iter.Seek([]byte(startWith)) {
			return to_return
		}
		if getPrev && !include {
			iter.Next()
		} else if !include {
			iter.Prev()
		}
	}

	for counter < limit {
		if len(iter.Key()) == 0 {
			if getPrev {
				iter.Next()
			} else {
				iter.Prev()
			}
			counter++
			continue
		}

		var logLine string
		if !caseSensetivity {
			logLine = strings.ToLower(string(iter.Value()))
			message = strings.ToLower(message)
		} else {
			logLine = string(iter.Value())
		}

		if !strings.Contains(logLine, message) {
			if getPrev {
				iter.Next()
			} else {
				iter.Prev()
			}
			continue
		}

		datetime := strings.Split(string(iter.Key()), " +")[0]
		to_return = append(
			to_return,
			[]string{
				datetime, string(iter.Value()),
			},
		)
		if getPrev {
			iter.Next()
		} else {
			iter.Prev()
		}
		counter++
	}

	defer iter.Release()
	return to_return
}

func DeleteContainer(host string, container string) {
	if vars.ActiveDBs[container] != nil {
		vars.ActiveDBs[container].Close()
	}
	os.RemoveAll("leveldb/hosts" + host + "/container/" + container)
}

// TODO remove all logs
func DeleteContainerLogs(host string, container string) {
	path := "leveldb/hosts" + host + "/container/" + container + "/logs"
	files, _ := os.ReadDir(path)
	lastNum := 0
	var lastName string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".log") {
			os.Remove(path + "/" + file.Name())
			continue
		}

		if !strings.HasSuffix(file.Name(), "ldb") {
			continue
		}

		num, _ := strconv.Atoi(file.Name()[:len(file.Name())-4])
		if num > lastNum {
			lastNum = num
			lastName = file.Name()
		}
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "ldb") || file.Name() == lastName {
			continue
		}

		os.Remove(path + "/" + file.Name())
	}
}
