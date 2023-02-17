package statistics

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestRunStatisticForHost(t *testing.T) {
	go RunStatisticForHost("Test")
	time.Sleep(1 * time.Second)
	if vars.Counters_For_Hosts_Last_30_Min["Test"] == nil {
		t.Error("No counter variable for host was created!")
	}
	vars.Counters_For_Hosts_Last_30_Min["Test"] = nil
}

func TestRunStatisticForContainer(t *testing.T) {
	go RunStatisticForHost("Test")
	go RunStatisticForContainer("Test", "TestContainer")
	time.Sleep(1 * time.Second)
	if vars.Counters_For_Containers_Last_30_Min["Test/TestContainer"] == nil {
		t.Error("No counter variable for container was created!")
	}
	if vars.Stat_Containers_DBs["Test/TestContainer"] == nil {
		t.Error("DB for stats wasn't created!")
	}
}

func TestGetStatisticsByService(t *testing.T) {
	vars.Counters_For_Containers_Last_30_Min["test/test"] = map[string]uint64{"error": 1, "debug": 2, "info": 3, "warn": 4, "other": 5}
	os.RemoveAll("leveldb/hosts/test/containers/test/statistics")
	statDB, _ := leveldb.OpenFile("leveldb/hosts/test/containers/test/statistics", nil)
	to_put, _ := json.Marshal(vars.Counters_For_Containers_Last_30_Min["test/test"])
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	statDB.Put([]byte(datetime), to_put, nil)
	statDB.Close()

	res := GetStatisticsByService("test", "test", 2)
	if res["debug"] != 4 || res["error"] != 2 ||
		res["info"] != 6 || res["other"] != 10 ||
		res["warn"] != 8 {
		t.Error("Wrong value!\n", res)
	}
}

func TestGetStatisticsByHost(t *testing.T) {
	vars.Counters_For_Hosts_Last_30_Min[util.GetHost()] = map[string]uint64{"error": 1, "debug": 2, "info": 3, "warn": 4, "other": 5}
	os.RemoveAll("leveldb/hosts/" + util.GetHost() + "/statistics")
	statDB, _ := leveldb.OpenFile("leveldb/hosts/"+util.GetHost()+"/statistics", nil)
	vars.Stat_Hosts_DBs[util.GetHost()] = statDB
	to_put, _ := json.Marshal(vars.Counters_For_Hosts_Last_30_Min[util.GetHost()])
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	statDB.Put([]byte(datetime), to_put, nil)

	res := GetStatisticsByHost(util.GetHost(), 2)
	if res["debug"] != 4 || res["error"] != 2 ||
		res["info"] != 6 || res["other"] != 10 ||
		res["warn"] != 8 {
		t.Error("Wrong value!")
	}
}

func TestGetChartData(t *testing.T) {
	cur_db, _ := leveldb.OpenFile("leveldb/hosts/test/statistics", nil)
	vars.Stat_Hosts_DBs["test"] = cur_db
	vars.Counters_For_Containers_Last_30_Min["test/test"] = map[string]uint64{"error": 2, "debug": 1, "info": 3, "warn": 5, "other": 4}
	to_put, _ := json.Marshal(vars.Counters_For_Containers_Last_30_Min["test/test"])
	datetime := strings.Replace(strings.Split(time.Now().UTC().String(), ".")[0], " ", "T", 1) + "Z"
	cur_db.Put([]byte(datetime), to_put, nil)

	res := GetChartData("test", "test", "hour", 2)
	datetime = datetime[:len(datetime)-6] + "00Z"
	if res[datetime]["debug"] != 1 || res[datetime]["error"] != 2 ||
		res[datetime]["info"] != 3 || res[datetime]["other"] != 4 ||
		res[datetime]["warn"] != 5 || res["now"]["debug"] != 1 ||
		res["now"]["error"] != 2 || res["now"]["info"] != 3 ||
		res["now"]["other"] != 4 || res["now"]["warn"] != 5 {
		t.Error("Wrong value!\n", res[datetime])
	}

}
