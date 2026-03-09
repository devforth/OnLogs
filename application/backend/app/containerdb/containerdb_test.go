package containerdb

import (
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestPutLogMessage(t *testing.T) {
	cont := "testCont"
	host := "testHost"
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + cont)
	vars.Container_Stat_Counter[host+"/"+cont] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+cont+"/logs", nil)
	statusDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+cont+"/statuses", nil)
	vars.Statuses_DBs[host+"/"+cont] = statusDB
	defer statusDB.Close()
	defer db.Close()

	PutLogMessage(db, host, cont, []string{vars.Year + "-02-10T12:56:09.230421754Z", "vokAU6OdSulJGynsz wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{vars.Year + "-02-10T12:57:09.230421754Z", "ERROR wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{vars.Year + "-02-10T12:58:09.230421754Z", "WARN vokAU6OdSulJGynsz\r\n"})
	PutLogMessage(db, host, cont, []string{vars.Year + "-02-10T12:59:09.230421754Z", "DEBUG wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{vars.Year + "-02-10T12:59:59.230421754Z", "INFO fasdfasdfB&^*inuk\r\n"})

	keys := []string{
		vars.Year + "-02-10T12:56:09.230421754Z", vars.Year + "-02-10T12:57:09.230421754Z",
		vars.Year + "-02-10T12:58:09.230421754Z", vars.Year + "-02-10T12:59:09.230421754Z",
		vars.Year + "-02-10T12:59:59.230421754Z",
	}
	for _, key := range keys {
		iter := db.NewIterator(nil, nil)
		has := false
		for iter.Next() {
			if strings.HasPrefix(string(iter.Key()), key+" +") {
				has = true
				break
			}
		}
		iter.Release()
		if !has {
			t.Error("Key is not in db: " + key)
		}
	}

	PutLogMessage(db, host, cont, []string{"123", "fasdf\r\n"})
	has, _ := db.Has([]byte("123"), nil)
	if has {
		t.Error("Bad key is in db!")
	}

	defer func() {
		r := recover().(string)
		if r != "Host is not mentioned!" {
			t.Error("Not expected error: ", r)
		}
	}()
	PutLogMessage(db, "", cont, []string{vars.Year + "-02-10T12:57:09.230421754Z", "fasdf\r\n"})
}

func TestGetLogs(t *testing.T) {
	_ = os.RemoveAll("leveldb/hosts/Test/containers/TestGetLogsCont")
	db, _ := leveldb.OpenFile("leveldb/hosts/Test/containers/TestGetLogsCont/logs", nil)
	vars.Container_Stat_Counter["Test/TestGetLogsCont"] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
	statusDB, _ := leveldb.OpenFile("leveldb/hosts/Test/containers/TestGetLogsCont/statuses", nil)
	vars.Statuses_DBs["Test/TestGetLogsCont"] = statusDB
	defer statusDB.Close()

	PutLogMessage(db, "Test", "TestGetLogsCont", []string{vars.Year + "-02-10T12:57:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{vars.Year + "-02-10T12:51:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{vars.Year + "-02-10T12:52:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{vars.Year + "-02-10T12:53:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{vars.Year + "-02-10T12:54:09.230421754Z", "fasdf\r\n"})
	db.Close()

	var logs [][]string
	logs = GetLogs(false, true, "Test", "TestGetLogsCont", "", 30, vars.Year+"-02-10T12:57:09.230421754Z", false, nil)["logs"].([][]string)
	if len(logs) != 5 {
		t.Error("5 logItems must be returned!")
	}
	if logs[0][0] != vars.Year+"-02-10T12:57:09.230421754Z" {
		t.Error("Invalid first logItem datetime: ", logs[0][0])
	}
	if logs[4][0] != vars.Year+"-02-10T12:51:09.230421754Z" {
		t.Error("Invalid last logItem datetime: ", logs[4][0])
	}

	logs = GetLogs(true, false, "Test", "TestGetLogsCont", "", 30, vars.Year+"-02-10T12:51:09.230421754Z", false, nil)["logs"].([][]string)
	if len(logs) != 4 {
		t.Error("4 logItems must be returned!")
	}
	if logs[0][0] != vars.Year+"-02-10T12:52:09.230421754Z" {
		t.Error("Invalid first logItem datetime: ", logs[0][0])
	}
	if logs[3][0] != vars.Year+"-02-10T12:57:09.230421754Z" {
		t.Error("Invalid last logItem datetime: ", logs[3][0])
	}

	logs = GetLogs(true, false, "Test", "TestGetLogsCont", "", 30, vars.Year+"-02-10T12:51:09.230421753Z", false, nil)["logs"].([][]string)
	if len(logs) != 5 {
		t.Error("4 logItems must be returned!")
	}
	if logs[0][0] != vars.Year+"-02-10T12:51:09.230421754Z" {
		t.Error("Invalid first logItem datetime: ", logs[0][0])
	}
	if logs[4][0] != vars.Year+"-02-10T12:57:09.230421754Z" {
		t.Error("Invalid last logItem datetime: ", logs[3][0])
	}
}

func TestFitsForSearchWithANSI(t *testing.T) {
	logLine := "\x1b[37mWARNING\x1b[2m AzL4Y8oR KsTdiwHodbZ0i \tmOK2Wz \x1b[0m"

	if !fitsForSearch(logLine, "WARNING Az", true) {
		t.Error("Expected query to match when ANSI escape codes are present")
	}

	if !fitsForSearch(logLine, "KsTdiwHodbZ0i m", true) {
		t.Error("Expected query to match across tab-separated boundary")
	}

	if fitsForSearch(logLine, "NOT_PRESENT", true) {
		t.Error("Expected non-existing query not to match")
	}
}

func TestPutLogMessageSameTimestampAcrossRestart(t *testing.T) {
	host := "RestartHost"
	container := "RestartContainer"
	location := host + "/" + container
	_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	vars.Container_Stat_Counter[location] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}

	db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	statusDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/statuses", nil)
	vars.Statuses_DBs[location] = statusDB

	ts := vars.Year + "-02-10T12:57:09.230421754Z"
	_ = PutLogMessage(db, host, container, []string{ts, "first"})
	_ = PutLogMessage(db, host, container, []string{ts, "second"})
	logKeyCounter.Store(0)
	_ = PutLogMessage(db, host, container, []string{ts, "third"})
	db.Close()
	statusDB.Close()

	checkDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	defer checkDB.Close()
	iter := checkDB.NewIterator(nil, nil)
	defer iter.Release()
	count := 0
	for iter.Next() {
		if strings.HasPrefix(string(iter.Key()), ts+" +") {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected 3 persisted keys with identical timestamp, got %d", count)
	}
}
