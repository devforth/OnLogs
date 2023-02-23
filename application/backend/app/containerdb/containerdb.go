package containerdb

import (
	"fmt"
	"os"
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

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) error {
	if len(message_item[0]) < 30 {
		fmt.Println("WARNING: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return nil
	}

	if host == "" {
		panic("Host is not mentioned!")
	}
	location := host + "/" + container

	if strings.Contains(message_item[1], "ERROR") || strings.Contains(message_item[1], "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_item[1], "Error") || strings.Contains(message_item[1], "Err") {
		vars.Counters_For_Containers_Last_30_Min[location]["error"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("error"), nil)

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

	return db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
}

func GetLogsByStatus(host string, container string, message string, status string, limit int, startWith string, getPrev bool, include bool, caseSensetivity bool) [][]string {
	logs_db := util.GetDB(host, container, "logs")
	db := util.GetDB(host, container, "statuses")
	if host != util.GetHost() || vars.ActiveDBs[container] == nil {
		defer logs_db.Close()
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

		logLineToCompare := logLine
		messageToCompare := message
		if !caseSensetivity {
			logLineToCompare = strings.ToLower(logLine)
			messageToCompare = strings.ToLower(message)
		}

		if !strings.Contains(logLineToCompare, messageToCompare) {
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
	db := util.GetDB(host, container, "logs")
	if host != util.GetHost() || vars.ActiveDBs[container] == nil {
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

func DeleteContainer(host string, container string, fullDelete bool) {
	if fullDelete {
		os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	} else {
		files, _ := os.ReadDir("leveldb/hosts/" + host + "/containers/" + container)
		for _, file := range files {
			os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container + "/" + file.Name())
		}
	}

	if vars.ActiveDBs[container] != nil {
		vars.ActiveDBs[container].Close()
		newActiveDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
		vars.ActiveDBs[container] = newActiveDB
	}
	if vars.Statuses_DBs[host+"/"+container] != nil {
		vars.Statuses_DBs[host+"/"+container].Close()
		newStatusesDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statuses", nil)
		vars.Statuses_DBs[host+"/"+container] = newStatusesDB
	}
	if vars.Stat_Containers_DBs[host+"/"+container] != nil {
		vars.Stat_Containers_DBs[host+"/"+container].Close()
		newStatDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statistics", nil)
		vars.Statuses_DBs[host+"/"+container] = newStatDB
	}
	vars.Counters_For_Containers_Last_30_Min[host+"/"+container] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
}
