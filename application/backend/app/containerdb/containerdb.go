package containerdb

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) error {
	if len(message_item[0]) < 30 {
		fmt.Println("WARNING: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return nil
	}

	if host == "" {
		panic("Host is not mentioned!")
	}
	location := host + "/" + container
	if vars.Statuses_DBs[location] == nil {
		vars.Statuses_DBs[location] = util.GetDB(host, container, "statuses")
	}

	status_key := "other"
	if strings.Contains(message_item[1], "ERROR") || strings.Contains(message_item[1], "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_item[1], "Error") || strings.Contains(message_item[1], "Err") {
		status_key = "error"
	} else if strings.Contains(message_item[1], "WARN") || strings.Contains(message_item[1], "WARNING") { // const statuses_warnings = ["WARN", "WARNING"];
		status_key = "warn"
	} else if strings.Contains(message_item[1], "DEBUG") { // const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
		status_key = "debug"
	} else if strings.Contains(message_item[1], "INFO") {
		status_key = "info"
	} else if strings.Contains(message_item[1], "ONLOGS") {
		status_key = "meta"
	}
	vars.Counters_For_Containers_Last_30_Min[location][status_key]++
	vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte(status_key), nil)

	err := db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
	tries := 0
	for err != nil && tries < 10 {
		db = util.GetDB(host, container, "logs")
		err = db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
		time.Sleep(10 * time.Millisecond)
		tries++
	}
	if err != nil {
		panic(err)
	}
	return err
}

func fitsForSearch(logLine string, message string, caseSensetivity bool) bool {
	if !caseSensetivity {
		logLine = strings.ToLower(logLine)
		message = strings.ToLower(message)
	}

	return strings.Contains(logLine, message)
}

func increaseAndMove(counter *int, move_direction func() bool) {
	*counter++
	move_direction()
}

func getMoveDirection(getPrev bool, iter iterator.Iterator) func() bool {
	if !getPrev {
		return func() bool { return iter.Prev() }
	}
	return func() bool { return iter.Next() }
}

func searchInit(iter iterator.Iterator, startWith string, getPrev bool, include bool, move_direction func() bool) bool {
	iter.Last()
	if startWith != "" {
		if !iter.Seek([]byte(startWith)) {
			return false
		}
		if !include {
			move_direction()
			return true
		}
	}
	return true
}

func getDateTimeFromKey(key string) string {
	return strings.Split(key, " +")[0]
}

/*
Get logs line by line from container.
  - getPrev - if true, will get logs from latest to oldest.
  - include - if true, will include logs with startWith key.

returns json obj like this:

	{
		"logs": [["2021-09-01T12:00:00", "logline"], ["2021-09-01T12:00:01", "logline"]],
		"last_processed_key": "2021-09-01T12:00:01",
		"is_end": false
	}
*/
func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool, status *string) map[string]interface{} {
	logs_db := util.GetDB(host, container, "logs")
	var statusDb *leveldb.DB
	if status != nil {
		statusDb = util.GetDB(host, container, "statuses")
	}
	iter := logs_db.NewIterator(nil, nil)
	defer iter.Release()

	to_return := map[string]interface{}{
		"logs": [][]string{},
	}
	logs := [][]string{}
	move_direction := getMoveDirection(getPrev, iter)

	if !searchInit(iter, startWith, getPrev, include, move_direction) {
		to_return["is_end"] = true
		return to_return
	}

	counter := 0
	iteration := 0
	last_processed_key := ""
	for counter < limit && iteration < 1000000 {
		iteration += 1
		key := iter.Key()
		if len(key) == 0 {
			to_return["is_end"] = true
			increaseAndMove(&counter, move_direction)
			continue
		} else {
			to_return["is_end"] = false
		}

		keyStr := string(key)
		value := string(iter.Value())

		if status != nil {
			statusValue, err := statusDb.Get(key, nil)
			if err != nil || string(statusValue) != *status {
				move_direction()
				continue
			}
		}

		if !fitsForSearch(value, message, caseSensetivity) {
			move_direction()
			continue
		}

		logs = append(logs, []string{getDateTimeFromKey(keyStr), value})
		increaseAndMove(&counter, move_direction)
		last_processed_key = keyStr
	}

	to_return["logs"] = logs
	to_return["last_processed_key"] = last_processed_key
	return to_return
}

func DeleteContainer(host string, container string, fullDelete bool) {
	path := "leveldb/hosts/" + host + "/containers/" + container
	if fullDelete {
		os.RemoveAll(path)
	} else {
		files, _ := os.ReadDir(path)
		for _, file := range files {
			os.RemoveAll(path + "/" + file.Name())
		}
	}

	if vars.ActiveDBs[container] != nil {
		vars.ActiveDBs[container].Close()
		vars.ActiveDBs[container] = util.GetDB(host, container, "active")
	}
	if vars.Statuses_DBs[host+"/"+container] != nil {
		vars.Statuses_DBs[host+"/"+container].Close()
		vars.Statuses_DBs[host+"/"+container] = util.GetDB(host, container, "statuses")
	}
	if vars.Stat_Containers_DBs[host+"/"+container] != nil {
		vars.Stat_Containers_DBs[host+"/"+container].Close()
		vars.Statuses_DBs[host+"/"+container] = util.GetDB(host, container, "statistics")
	}
	vars.Counters_For_Containers_Last_30_Min[host+"/"+container] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
}
