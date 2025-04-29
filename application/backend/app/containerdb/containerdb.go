package containerdb

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
)

func GetLogStatusKey(message string) string {
	if strings.Contains(message, "ERROR") || strings.Contains(message, "ERR") {
		return "error"
	} else if strings.Contains(message, "WARN") || strings.Contains(message, "WARNING") {
		return "warn"
	} else if strings.Contains(message, "DEBUG") {
		return "debug"
	} else if strings.Contains(message, "INFO") {
		return "info"
	} else if strings.Contains(message, "ONLOGS") {
		return "meta"
	}
	return "other"
}

func checkAndManageLogSize(host string, container string) error {
	maxSize, err := util.ParseHumanReadableSize(os.Getenv("MAX_LOGS_SIZE"))
	if err != nil {
		return fmt.Errorf("failed to parse MAX_LOGS_SIZE: %v", err)
	}

	for {
		hosts, err := os.ReadDir("leveldb/hosts/")
		if err != nil {
			return fmt.Errorf("failed to read hosts directory: %v", err)
		}

		var totalSize int64
		for _, h := range hosts {
			hostName := h.Name()
			containers, _ := os.ReadDir("leveldb/hosts/" + hostName + "/containers")
			for _, c := range containers {
				containerName := c.Name()
				size := util.GetDirSize(hostName, containerName)
				totalSize += int64(size * 1024 * 1024)
			}
		}

		fmt.Printf("Max size: %d, current dir size: %d\n", maxSize, totalSize)
		if totalSize <= maxSize {
			break
		}

		var cutoffKeys [][]byte
		for _, h := range hosts {
			hostName := h.Name()
			containers, _ := os.ReadDir("leveldb/hosts/" + hostName + "/containers")
			for _, c := range containers {
				containerName := c.Name()
				logsDB := util.GetDB(hostName, containerName, "logs")
				if logsDB == nil {
					continue
				}

				cutoffKeysForContainer, err := getCutoffKeysForContainer(logsDB, 200)
				if err != nil || len(cutoffKeysForContainer) == 0 {
					continue
				}
				cutoffKeys = append(cutoffKeys, cutoffKeysForContainer)
			}
		}

		if len(cutoffKeys) == 0 {
			fmt.Println("Nothing to delete, cutoff keys not found.")
			break
		}

		oldestCutoffKey := findOldestCutoffKey(cutoffKeys)
		oldestTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(oldestCutoffKey)))
		if err != nil {
			fmt.Println("Error parsing oldest time:", err)
			break
		}
		fmt.Println("Oldest time for deletion cutoff:", oldestTime)

		for _, h := range hosts {
			hostName := h.Name()
			containers, _ := os.ReadDir("leveldb/hosts/" + hostName + "/containers")
			for _, c := range containers {
				containerName := c.Name()
				logsDB := util.GetDB(hostName, containerName, "logs")
				if logsDB == nil {
					continue
				}

				batch := new(leveldb.Batch)
				deletedCount := 0
				iter := logsDB.NewIterator(nil, nil)

				count := 0
				for ok := iter.First(); ok && count < 200; ok = iter.Next() {
					count++
					keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(iter.Key())))
					if err != nil {
						fmt.Println("Error parsing key time:", err)
						continue
					}
					if keyTime.Before(oldestTime) || keyTime.Equal(oldestTime) {
						batch.Delete(iter.Key())
						deletedCount++
					}
				}
				iter.Release()

				if deletedCount > 0 {
					err = logsDB.Write(batch, nil)
					if err != nil {
						fmt.Printf("Failed to delete batch in %s/%s: %v\n", hostName, containerName, err)
					} else {
						fmt.Printf("Deleted %d logs from %s/%s\n", deletedCount, hostName, containerName)
					}
					logsDB.CompactRange(leveldbUtil.Range{Start: nil, Limit: nil})
				}

				statusesDB := util.GetDB(hostName, containerName, "statuses")
				if statusesDB != nil {
					batch := new(leveldb.Batch)
					deletedCountStatuses := 0
					iter := statusesDB.NewIterator(nil, nil)

					for ok := iter.First(); ok; ok = iter.Next() {
						keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(iter.Key())))
						if err != nil {
							fmt.Println("Error parsing key time:", err)
							continue
						}
						if keyTime.Before(oldestTime) || keyTime.Equal(oldestTime) {
							batch.Delete(iter.Key())
							deletedCountStatuses++
						}
					}
					iter.Release()

					if deletedCountStatuses > 0 {
						err := statusesDB.Write(batch, nil)
						if err != nil {
							fmt.Printf("Failed to delete batch in statusesDB for %s/%s: %v\n", hostName, containerName, err)
						}
						statusesDB.CompactRange(leveldbUtil.Range{Start: nil, Limit: nil})
					}
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func getCutoffKeysForContainer(db *leveldb.DB, limit int) ([]byte, error) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	var cutoffKeys [][]byte
	for ok := iter.First(); ok && len(cutoffKeys) < limit; ok = iter.Next() {
		key := append([]byte{}, iter.Key()...)
		cutoffKeys = append(cutoffKeys, key)
	}

	if len(cutoffKeys) < limit {
		return nil, fmt.Errorf("insufficient records to form cutoff keys")
	}

	return cutoffKeys[len(cutoffKeys)-1], nil
}

func findOldestCutoffKey(cutoffKeys [][]byte) []byte {
	var oldestKey []byte
	var oldestTime time.Time
	first := true

	for _, key := range cutoffKeys {
		keyStr := string(key)
		keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(keyStr))
		if err != nil {
			fmt.Println("Error parsing key time:", err)
			continue
		}

		if first || keyTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = keyTime
			first = false
		}
	}
	return oldestKey
}

var (
	logCleanupMu     sync.Mutex
	nextCleanup      time.Time
	isCleanupRunning bool
)

func MaybeScheduleCleanup(host string, container string) {
	logCleanupMu.Lock()

	defer logCleanupMu.Unlock()

	if isCleanupRunning {
		return
	}
	if time.Now().Before(nextCleanup) {
		return
	}

	isCleanupRunning = true

	go func() {
		err := checkAndManageLogSize(host, container)

		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()

		isCleanupRunning = false
		nextCleanup = time.Now().Add(1 * time.Minute)

		if err != nil {
			fmt.Printf("Log cleanup failed: %v\n", err)
		}
	}()
}

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) error {
	if len(message_item[0]) < 30 {
		fmt.Println("WARNING: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return nil
	}

	if host == "" {
		panic("Host is not mentioned!")
	}

	MaybeScheduleCleanup(host, container)

	location := host + "/" + container
	if vars.Statuses_DBs[location] == nil {
		vars.Statuses_DBs[location] = util.GetDB(host, container, "statuses")
	}
	status_key := GetLogStatusKey(message_item[1])
	vars.Mutex.Lock()
	vars.Container_Stat_Counter[location][status_key]++
	vars.Mutex.Unlock()
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

func searchInit(iter iterator.Iterator, startWith string) bool {
	iter.Last()

	if startWith != "" {
		if !iter.Seek([]byte(startWith)) {
			return startWith > getDateTimeFromKey(string(iter.Key()))
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

	if !searchInit(iter, startWith) {
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
		timeStr := getDateTimeFromKey(keyStr)
		if !include && timeStr == startWith {
			move_direction()
			continue
		}
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

		logs = append(logs, []string{timeStr, value})
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

	vars.Mutex.Lock()
	vars.Container_Stat_Counter[host+"/"+container] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	vars.Mutex.Unlock()
}
