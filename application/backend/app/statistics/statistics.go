package statistics

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func restartStats(host string, container string, current_db *leveldb.DB) {
	var used_storage map[string]map[string]uint64
	var location string
	if container == "" {
		used_storage = vars.Counters_For_Hosts_Last_30_Min
		location = host
	} else {
		used_storage = vars.Counters_For_Containers_Last_30_Min
		location = host + "/" + container
	}
	copy := map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	copy["error"] = used_storage[location]["error"]
	copy["debug"] = used_storage[location]["debug"]
	copy["info"] = used_storage[location]["info"]
	copy["warn"] = used_storage[location]["warn"]
	copy["other"] = used_storage[location]["other"]
	to_put, _ := json.Marshal(copy)
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	current_db.Put([]byte(datetime), to_put, nil)

	used_storage[location]["error"] = 0
	used_storage[location]["debug"] = 0
	used_storage[location]["info"] = 0
	used_storage[location]["warn"] = 0
	used_storage[location]["other"] = 0
}

func RunStatisticForContainer(host string, container string) {
	location := host + "/" + container
	vars.Counters_For_Containers_Last_30_Min[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	if vars.Stat_Containers_DBs[location] == nil {
		current_db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statistics", nil)
		defer current_db.Close()
		vars.Stat_Containers_DBs[location] = current_db
	}
	defer delete(vars.Stat_Containers_DBs, location)
	defer restartStats(host, container, vars.Stat_Containers_DBs[location])
	for {
		vars.Counters_For_Hosts_Last_30_Min[host]["error"] += vars.Counters_For_Containers_Last_30_Min[location]["error"]
		vars.Counters_For_Hosts_Last_30_Min[host]["debug"] += vars.Counters_For_Containers_Last_30_Min[location]["debug"]
		vars.Counters_For_Hosts_Last_30_Min[host]["info"] += vars.Counters_For_Containers_Last_30_Min[location]["info"]
		vars.Counters_For_Hosts_Last_30_Min[host]["warn"] += vars.Counters_For_Containers_Last_30_Min[location]["warn"]
		vars.Counters_For_Hosts_Last_30_Min[host]["other"] += vars.Counters_For_Containers_Last_30_Min[location]["other"]
		restartStats(host, container, vars.Stat_Containers_DBs[location])
		time.Sleep(30 * time.Minute)
	}
}

// TODO improve counters
func RunStatisticForHost(host string) {
	vars.Counters_For_Hosts_Last_30_Min[host] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	if vars.Stat_Hosts_DBs[host] == nil {
		current_db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/statistics", nil)
		defer current_db.Close()
		vars.Stat_Hosts_DBs[host] = current_db
	}
	defer delete(vars.Stat_Hosts_DBs, host)
	defer restartStats(host, "", vars.Stat_Hosts_DBs[host])
	for {
		restartStats(host, "", vars.Stat_Hosts_DBs[host])
		time.Sleep(30 * time.Minute)
	}
}
