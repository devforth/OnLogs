package containerdb

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func containStr(a string, b string, caseSens bool) bool {
	if caseSens {
		return strings.Contains(a, b)
	}

	return strings.Contains(strings.ToLower(a), strings.ToLower(b))
}

func getDB(host string, container string, dbType string) *leveldb.DB {
	var res_db *leveldb.DB
	if dbType == "logs" {
		res_db = vars.ActiveDBs[container]
	} else if dbType == "statuses" {
		res_db = vars.Statuses_DBs[host+"/"+container]
	}

	var err error
	if res_db == nil {
		path := "leveldb/hosts/" + host + "/containers/" + container + "/" + dbType

		_, pathErr := os.Stat(path)
		if os.IsNotExist(pathErr) {
			return nil
		}

		res_db, err = leveldb.OpenFile(path, nil)
		if err != nil {
			res_db, _ = leveldb.RecoverFile(path, nil)
		}
	}
	return res_db
}

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) {
	if len(message_item[0]) < 30 {
		fmt.Println("ERROR: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return
	}

	if host == "" {
		panic("Host is not mentioned!")
	}
	location := host + "/" + container

	if strings.Contains(message_item[1], "ERROR") || strings.Contains(message_item[1], "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_item[1], "Error") || strings.Contains(message_item[1], "Err") {
		vars.Counters_For_Containers_Last_30_Min[location]["error"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("err"), nil)

	} else if strings.Contains(message_item[1], "WARN") || strings.Contains(message_item[1], "WARNING") { // const statuses_warnings = ["WARN", "WARNING"];
		vars.Counters_For_Containers_Last_30_Min[location]["warn"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("warn"), nil)

	} else if strings.Contains(message_item[1], "DEBUG") { // const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
		vars.Counters_For_Containers_Last_30_Min[location]["debug"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("debug"), nil)

	} else if strings.Contains(message_item[1], "INFO") {
		vars.Counters_For_Containers_Last_30_Min[location]["info"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("info"), nil)

	} else {
		vars.Counters_For_Containers_Last_30_Min[location]["other"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("other"), nil)
	}

	db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
}

func GetLogsByStatus(host string, container string, message string, status string, limit int, startWith string, getPrev bool, include bool, caseSensetivity bool) [][]string {
	logs_db := getDB(host, container, "logs")
	db := getDB(host, container, "statuses")

	iter := db.NewIterator(nil, nil)
	defer iter.Release()
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

		if string(iter.Value()) != status {
			if getPrev {
				iter.Next()
			} else {
				iter.Prev()
			}
			continue
		}

		res, _ := logs_db.Get(iter.Key(), nil)
		logLine := string(res)

		if !caseSensetivity {
			logLine = strings.ToLower(logLine)
			message = strings.ToLower(message)
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
		to_return = append(to_return, []string{datetime, logLine})
		if getPrev {
			iter.Next()
		} else {
			iter.Prev()
		}
		counter++
	}

	return to_return
}

func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool) [][]string {
	db := getDB(host, container, "logs")
	if host != util.GetHost() {
		defer db.Close()
	}

	iter := db.NewIterator(nil, nil)
	defer iter.Release()
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
