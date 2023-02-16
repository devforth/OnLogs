package containerdb

import (
	"testing"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func Test_containStr(t *testing.T) {
	type args struct {
		a        string
		b        string
		caseSens bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Not contain", args{a: "Amogus", b: "sus", caseSens: false}, false},
		{"Contain without caseSens", args{a: "Amogus", b: "O", caseSens: false}, true},
		{"Contain, but caseSens", args{a: "Amogus", b: "O", caseSens: true}, false},
		{"Contain", args{a: "Amogus", b: "o", caseSens: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containStr(tt.args.a, tt.args.b, tt.args.caseSens); got != tt.want {
				t.Errorf("containStr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPutLogMessage(t *testing.T) {
	cont := "testCont"
	host := "testHost"
	vars.Counters_For_Containers_Last_30_Min[host+"/"+cont] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	db, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+cont+"/logs", nil)
	statusDB, _ := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+cont+"statuses", nil)
	vars.Statuses_DBs[host+"/"+cont] = statusDB
	defer statusDB.Close()
	defer db.Close()

	PutLogMessage(db, host, cont, []string{"fasd2023-02-10T12:56:09.230421754Z", "vokAU6OdSulJGynsz wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{"2023-02-10T12:57:09.230421754Z", "ERROR wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{"2023-02-10T12:58:09.230421754Z", "WARN vokAU6OdSulJGynsz\r\n"})
	PutLogMessage(db, host, cont, []string{"2023-02-10T12:59:09.230421754Z", "DEBUG wBaKssXuAPGk6ZFiQxq4sQHe7B9Q9RbTAy\r\n"})
	PutLogMessage(db, host, cont, []string{"2023-02-10T12:59:59.230421754Z", "INFO fasdfasdfB&^*inuk\r\n"})

	keys := []string{
		"2023-02-10T12:56:09.230421754Z", "2023-02-10T12:57:09.230421754Z",
		"2023-02-10T12:58:09.230421754Z", "2023-02-10T12:59:09.230421754Z",
		"2023-02-10T12:59:59.230421754Z",
	}
	for _, key := range keys {
		has, _ := db.Has([]byte(key), nil)
		if !has {
			t.Error("Key is not in db: ", key)
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
	PutLogMessage(db, "", cont, []string{"2023-02-10T12:57:09.230421754Z", "fasdf\r\n"})
}

func TestGetLogs(t *testing.T) {
	db, _ := leveldb.OpenFile("leveldb/hosts/Test/containers/TestGetLogsCont/logs", nil)
	vars.Counters_For_Containers_Last_30_Min["Test/TestGetLogsCont"] = map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "other": 0}
	statusDB, _ := leveldb.OpenFile("leveldb/hosts/Test/containers/TestGetLogsCont/statuses", nil)
	vars.Statuses_DBs["Test/TestGetLogsCont"] = statusDB
	defer statusDB.Close()

	PutLogMessage(db, "Test", "TestGetLogsCont", []string{"2023-02-10T12:57:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{"2023-02-10T12:51:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{"2023-02-10T12:52:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{"2023-02-10T12:53:09.230421754Z", "fasdf\r\n"})
	PutLogMessage(db, "Test", "TestGetLogsCont", []string{"2023-02-10T12:54:09.230421754Z", "fasdf\r\n"})
	db.Close()
	logs := GetLogs(false, true, "Test", "TestGetLogsCont", "", 30, "2023-02-10T12:57:09.230421754Z", false)
	if len(logs) != 5 {
		t.Error("5 logItems must be returned!")
	}
	if logs[0][0] != "2023-02-10T12:57:09.230421754Z" {
		t.Error("Invalid first logItem datetime: ", logs[0][0])
	}

	logs = GetLogs(true, false, "Test", "TestGetLogsCont", "", 30, "2023-02-10T12:51:09.230421754Z", false)
	if len(logs) != 4 {
		t.Error("4 logItems must be returned!")
	}
	if logs[0][0] != "2023-02-10T12:52:09.230421754Z" {
		t.Error("Invalid first logItem datetime: ", logs[0][0])
	}
}
