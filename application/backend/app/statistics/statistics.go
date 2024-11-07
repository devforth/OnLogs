package statistics

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func restartStats(host string, container string) {
	current_db := util.GetDB(host, container, "statistics")
	location := host
	if container != "" {
		location += "/" + container
	}

	current_datetime := time.Now().UTC().Format("2006-01-02T15:04:05.999999999Z")

	last_stat_time := getLastStatTime(current_db)
	if last_stat_time == "" {
		last_stat_time = current_datetime
		calc_stat := collectLogsBackward(host, container, last_stat_time)
		saveStats(current_db, calc_stat, last_stat_time)
	} else {
		calc_stat := collectLogsForward(host, container, last_stat_time, current_datetime)
		saveStats(current_db, calc_stat, current_datetime)
	}

	resetInMemoryStats(location)
}

func getLastStatTime(db *leveldb.DB) string {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	if !iter.Last() {
		return ""
	}

	return string(iter.Key())
}

func collectLogsBackward(host, container, until string) map[string]uint64 {
	calc_stat := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}

	for {
		raw_logs := containerdb.GetLogs(false, false, host, container, "", 1000, until, true, nil)
		logs := raw_logs["logs"].([][]string)
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			status_key := containerdb.GetLogStatusKey(log[1])
			calc_stat[status_key]++
		}

		if raw_logs["is_end"].(bool) {
			break
		}
		until = raw_logs["last_processed_key"].(string)
	}

	return calc_stat
}

func collectLogsForward(host, container, since, until string) map[string]uint64 {
	calc_stat := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}

	raw_logs := containerdb.GetLogs(true, true, host, container, "", 1000, since, true, nil)
	if len(raw_logs["logs"].([][]string)) > 0 {
		logs := raw_logs["logs"].([][]string)
		for _, log := range logs {
			if log[0] >= until {
				return calc_stat
			}
			status_key := containerdb.GetLogStatusKey(log[1])
			calc_stat[status_key]++
		}
		since = raw_logs["last_processed_key"].(string)

		for !raw_logs["is_end"].(bool) {
			raw_logs = containerdb.GetLogs(true, false, host, container, "", 1000, since, true, nil)
			logs = raw_logs["logs"].([][]string)
			if len(logs) == 0 {
				break
			}

			for _, log := range logs {
				if log[0] >= until {
					return calc_stat
				}
				status_key := containerdb.GetLogStatusKey(log[1])
				calc_stat[status_key]++
			}

			since = raw_logs["last_processed_key"].(string)
		}
	}

	return calc_stat
}

func saveStats(db *leveldb.DB, stats map[string]uint64, timestamp string) {
	to_put, _ := json.Marshal(stats)
	db.Put([]byte(timestamp), to_put, nil)
}

func resetInMemoryStats(location string) {
	vars.Mutex.Lock()
	vars.Container_Stat_Counter[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	vars.Mutex.Unlock()
}

func RunStatisticForContainer(host string, container string) {
	location := host + "/" + container
	vars.Mutex.Lock()
	vars.Container_Stat_Counter[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	vars.Mutex.Unlock()
	defer restartStats(host, container)
	for {
		time.Sleep(vars.StatisticsSaveInterval)
		restartStats(host, container)
	}
}

func GetStatisticsByService(host string, service string, value int) map[string]uint64 {
	location := host + "/" + service

	vars.Mutex.Lock()
	to_return := vars.Container_Stat_Counter[location]
	vars.Mutex.Unlock()

	if to_return == nil {
		to_return = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	}

	if value < 1 {
		return to_return
	}

	searchTo := time.Now().Add(-(time.Hour * time.Duration(value/2))).UTC()
	var tmp_stats map[string]uint64
	current_db := util.GetDB(host, service, "statistics")
	iter := current_db.NewIterator(nil, nil)
	defer iter.Release()
	iter.Last()
	hasPrev := true
	result_map := map[string]uint64{"debug": to_return["debug"], "error": to_return["error"], "info": to_return["info"], "warn": to_return["warn"], "meta": to_return["meta"], "other": to_return["other"]}
	for hasPrev {
		tmp_time, _ := time.Parse(time.RFC3339Nano, string(iter.Key()))
		if searchTo.After(tmp_time) {
			break
		}
		json.Unmarshal(iter.Value(), &tmp_stats)
		result_map["debug"] += tmp_stats["debug"]
		result_map["error"] += tmp_stats["error"]
		result_map["info"] += tmp_stats["info"]
		result_map["warn"] += tmp_stats["warn"]
		result_map["meta"] += tmp_stats["meta"]
		result_map["other"] += tmp_stats["other"]
		hasPrev = iter.Prev()
	}
	return result_map
}

func GetChartData(host string, service string, unit string, uAmount int) map[string]map[string]uint64 {
	var searchTo time.Time
	var sep, formatting string
	if unit == "hour" {
		searchTo = time.Now().Add(-(time.Hour * time.Duration(uAmount)))
		sep = ":"
		formatting = ":00Z"
	} else if unit == "day" {
		searchTo = time.Now().AddDate(0, 0, -uAmount)
		sep = "T"
		formatting = "T00:00Z"
	} else if unit == "month" {
		searchTo = time.Now().AddDate(0, -uAmount, 0)
		formatting = "-01T00:00Z"
	} else {
		return nil
	}

	location := host + "/" + service
	to_return := map[string]map[string]uint64{}
	iter := util.GetDB(host, service, "statistics").NewIterator(nil, nil)
	iter.Last()
	defer iter.Release()
	hasPrev := true
	for hasPrev {
		tmp_time, _ := time.Parse(time.RFC3339Nano, string(iter.Key()))
		if searchTo.After(tmp_time) {
			break
		}

		var datetime string
		if unit == "month" {
			datetime = string(iter.Key())[:7] + formatting
		} else {
			datetime = strings.Split(string(iter.Key()), sep)[0] + formatting
		}
		to_return[datetime] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
		tmp_stats := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
		json.Unmarshal(iter.Value(), &tmp_stats)

		to_return[datetime]["error"] += tmp_stats["error"]
		to_return[datetime]["debug"] += tmp_stats["debug"]
		to_return[datetime]["info"] += tmp_stats["info"]
		to_return[datetime]["warn"] += tmp_stats["warn"]
		to_return[datetime]["meta"] += tmp_stats["meta"]
		to_return[datetime]["other"] += tmp_stats["other"]

		hasPrev = iter.Prev()
	}

	vars.Mutex.Lock()
	to_return["now"] = vars.Container_Stat_Counter[location]
	vars.Mutex.Unlock()

	return to_return
}
