package statistics

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

func restartStats(host string, container string) {
	current_db := util.GetDB(host, container, "statistics")
	location := host
	if container != "" {
		location += "/" + container
	}

	vars.Mutex.Lock()
	copy := vars.Counters_For_Containers_Last_30_Min[location]

	to_put, _ := json.Marshal(copy)
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	current_db.Put([]byte(datetime), to_put, nil)

	vars.Counters_For_Containers_Last_30_Min[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	vars.Mutex.Unlock()
}

func RunStatisticForContainer(host string, container string) {
	location := host + "/" + container
	vars.Mutex.Lock()
	vars.Counters_For_Containers_Last_30_Min[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	vars.Mutex.Unlock()
	defer restartStats(host, container)
	for {
		restartStats(host, container)
		time.Sleep(30 * time.Minute)
	}
}

func GetStatisticsByService(host string, service string, value int) map[string]uint64 {
	location := host + "/" + service

	vars.Mutex.Lock()
	to_return := vars.Counters_For_Containers_Last_30_Min[location]
	vars.Mutex.Unlock()

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
	for hasPrev {
		tmp_time, err := time.Parse(time.RFC3339, string(iter.Key()))
		if err != nil { // TODO no errors should be here, so this may be removed
			current_db.Delete(iter.Key(), nil)
		}
		if searchTo.After(tmp_time) {
			break
		}

		json.Unmarshal(iter.Value(), &tmp_stats)
		to_return["debug"] += tmp_stats["debug"]
		to_return["error"] += tmp_stats["error"]
		to_return["info"] += tmp_stats["info"]
		to_return["warn"] += tmp_stats["warn"]
		to_return["meta"] += tmp_stats["meta"]
		to_return["other"] += tmp_stats["other"]
		hasPrev = iter.Prev()
	}

	return to_return
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
	iter := vars.Stat_Containers_DBs[location].NewIterator(nil, nil)
	iter.Last()
	defer iter.Release()
	hasPrev := true
	for hasPrev {
		tmp_time, _ := time.Parse(time.RFC3339, string(iter.Key()))
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
	to_return["now"] = vars.Counters_For_Containers_Last_30_Min[location]
	vars.Mutex.Unlock()

	return to_return
}
