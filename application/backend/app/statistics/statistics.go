package statistics

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func restartStats(host string, container string, current_db *leveldb.DB) {
	var used_storage map[string]map[string]uint64
	var location string
	if container == "" {
		location = host
	} else {
		used_storage = vars.Counters_For_Containers_Last_30_Min
		location = host + "/" + container
	}
	copy := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	copy["error"] = used_storage[location]["error"]
	copy["debug"] = used_storage[location]["debug"]
	copy["info"] = used_storage[location]["info"]
	copy["warn"] = used_storage[location]["warn"]
	copy["meta"] = used_storage[location]["meta"]
	copy["other"] = used_storage[location]["other"]
	to_put, _ := json.Marshal(copy)
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	current_db.Put([]byte(datetime), to_put, nil)

	used_storage[location]["error"] = 0
	used_storage[location]["debug"] = 0
	used_storage[location]["info"] = 0
	used_storage[location]["warn"] = 0
	used_storage[location]["meta"] = 0
	used_storage[location]["other"] = 0
}

func RunStatisticForContainer(host string, container string) {
	location := host + "/" + container
	vars.Counters_For_Containers_Last_30_Min[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	if vars.Stat_Containers_DBs[location] == nil {
		current_db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statistics", nil)
		defer current_db.Close()
		vars.Stat_Containers_DBs[location] = current_db
	}
	defer delete(vars.Stat_Containers_DBs, location)
	defer restartStats(host, container, vars.Stat_Containers_DBs[location])
	for {
		restartStats(host, container, vars.Stat_Containers_DBs[location])
		time.Sleep(30 * time.Minute)
	}
}

func GetStatisticsByService(host string, service string, value int) map[string]uint64 {
	location := host + "/" + service

	to_return := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	to_return["debug"] += vars.Counters_For_Containers_Last_30_Min[location]["debug"]
	to_return["error"] += vars.Counters_For_Containers_Last_30_Min[location]["error"]
	to_return["info"] += vars.Counters_For_Containers_Last_30_Min[location]["info"]
	to_return["warn"] += vars.Counters_For_Containers_Last_30_Min[location]["warn"]
	to_return["meta"] += vars.Counters_For_Containers_Last_30_Min[location]["meta"]
	to_return["other"] += vars.Counters_For_Containers_Last_30_Min[location]["other"]

	if value < 1 {
		return to_return
	}

	searchTo := time.Now().Add(-(time.Hour * time.Duration(value/2))).UTC()
	var tmp_stats map[string]uint64
	current_db := util.GetDB(host, service, "statistics")
	if vars.Stat_Containers_DBs[location] == nil {
		defer current_db.Close()
	}
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

	to_return["now"] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	to_return["now"]["error"] = vars.Counters_For_Containers_Last_30_Min[location]["error"]
	to_return["now"]["debug"] = vars.Counters_For_Containers_Last_30_Min[location]["debug"]
	to_return["now"]["info"] = vars.Counters_For_Containers_Last_30_Min[location]["info"]
	to_return["now"]["warn"] = vars.Counters_For_Containers_Last_30_Min[location]["warn"]
	to_return["now"]["meta"] = vars.Counters_For_Containers_Last_30_Min[location]["meta"]
	to_return["now"]["other"] = vars.Counters_For_Containers_Last_30_Min[location]["other"]

	return to_return
}
