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
	if vars.Statuses_DBs[location] == nil {
		vars.Statuses_DBs[location] = util.GetDB(host, container, "statuses")
	}

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

	} else if strings.Contains(message_item[1], "ONLOGS") {
		vars.Counters_For_Containers_Last_30_Min[location]["meta"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("meta"), nil)
	} else {
		vars.Counters_For_Containers_Last_30_Min[location]["other"]++
		vars.Statuses_DBs[location].Put([]byte(message_item[0]), []byte("other"), nil)
	}

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
	if getPrev {
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

// # TODO: should be merged with GetLogs function
/*
Get logs line by line with filtering by logline status.
  - getPrev - if true, will get logs from latest to oldest.
  - include - if true, will include logs with startWith key.

returns json obj same to GetLogs function.
*/
func GetLogsByStatus(host string, container string, message string, status string, limit int, startWith string, getPrev bool, include bool, caseSensetivity bool) map[string]interface{} {
	logs_db := util.GetDB(host, container, "logs")
	db := util.GetDB(host, container, "statuses")
	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	to_return := map[string]interface{}{}
	to_return["logs"] = [][]string{}
	move_direction := getMoveDirection(getPrev, iter)

	if !searchInit(iter, startWith, getPrev, include, move_direction) {
		to_return["is_end"] = true
		return to_return
	}

	counter := 0
	iteration := 0
	last_processed_key := []string{}
	for counter < limit && iteration < 10000 {
		iteration += 1
		key := iter.Key()
		if len(key) == 0 {
			to_return["is_end"] = true
			increaseAndMove(&counter, move_direction)
			continue
		} else {
			to_return["is_end"] = false
		}

		value := string(iter.Value())
		if value != status {
			move_direction()
			continue
		}

		last_processed_key = []string{string(key), value}
		res, _ := logs_db.Get(key, nil)
		logLine := string(res)

		if !fitsForSearch(logLine, message, caseSensetivity) {
			move_direction()
			continue
		}

		to_return["logs"] = append(to_return["logs"].([][]string), []string{getDateTimeFromKey(string(key)), logLine})
		increaseAndMove(&counter, move_direction)
	}

	to_return["last_processed_key"] = last_processed_key
	return to_return
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
func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool) map[string]interface{} {
	logs_db := util.GetDB(host, container, "logs")
	iter := logs_db.NewIterator(nil, nil)
	defer iter.Release()
	to_return := map[string]interface{}{}
	to_return["logs"] = [][]string{}
	move_direction := getMoveDirection(getPrev, iter)

	if !searchInit(iter, startWith, getPrev, include, move_direction) {
		to_return["is_end"] = true
		return to_return
	}

	counter := 0
	iteration := 0
	last_processed_key := ""
	for counter < limit && iteration < 10000 {
		iteration += 1
		key := string(iter.Key())
		if len(key) == 0 {
			to_return["is_end"] = true
			increaseAndMove(&counter, move_direction)
			continue
		} else {
			to_return["is_end"] = false
		}
		last_processed_key = key
		value := string(iter.Value())

		if !fitsForSearch(value, message, caseSensetivity) {
			increaseAndMove(&counter, move_direction)
			continue
		}

		to_return["logs"] = append(to_return["logs"].([][]string), []string{getDateTimeFromKey(key), value})
		increaseAndMove(&counter, move_direction)
	}

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
