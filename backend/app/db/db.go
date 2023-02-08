package db

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	vars "github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func containStr(a string, b string, caseSens bool) bool {
	if caseSens {
		if strings.Contains(a, b) {
			return true
		}
		return false
	}
	if strings.Contains(strings.ToLower(a), strings.ToLower(b)) {
		return true
	}
	return false
}

func IsUserExists(login string) bool {
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	return isExists
}

func IsTokenExists(token string) bool {
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)

	iter := tokensDB.NewIterator(nil, nil)
	iter.First()
	if string(iter.Key()) == token {
		tokensDB.Put([]byte(token), []byte("was used"), nil)
		return true
	}
	for iter.Next() {
		if string(iter.Key()) == token {
			tokensDB.Put([]byte(token), []byte("was used"), nil)
			return true
		}
	}
	defer iter.Release()
	defer tokensDB.Close()

	return false
}

func CreateOnLogsToken() string {
	tokenLen := 25
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)
	defer tokensDB.Close()

	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*()_+,.:'{}[]"
	b := make([]byte, tokenLen)

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for i := range b {
		b[i] = letterBytes[r1.Int63()%int64(len(letterBytes))]
	}
	token := string(b)

	to_put := time.Now().UTC().Add(24 * time.Hour).String()
	tokensDB.Put(b, []byte(to_put), nil)
	return token
}

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) {
	if len(message_item[0]) < 30 {
		fmt.Println("ERROR: got broken timestamp: ", message_item)
		return
	}

	for !strings.HasPrefix(message_item[0], "2") && len(message_item[0]) > 20 {
		message_item[0] = message_item[0][1:]
	}

	if host == "" {
		panic("Host is not mentioned!")
	}
	location := host + "/" + container

	if strings.Contains(message_item[1], "ERROR") || strings.Contains(message_item[1], "ERR") || // const statuses_errors = ["ERROR", "ERR", "Error", "Err"];
		strings.Contains(message_item[1], "Error") || strings.Contains(message_item[1], "Err") {
		vars.Counters_For_Containers_Last_30_Min[location]["error"]++

	} else if strings.Contains(message_item[1], "WARN") || strings.Contains(message_item[1], "WARNING") { // const statuses_warnings = ["WARN", "WARNING"];
		vars.Counters_For_Containers_Last_30_Min[location]["warn"]++

	} else if strings.Contains(message_item[1], "DEBUG") { // const statuses_other = ["DEBUG", "INFO", "ONLOGS"];
		vars.Counters_For_Containers_Last_30_Min[location]["debug"]++

	} else if strings.Contains(message_item[1], "INFO") {
		vars.Counters_For_Containers_Last_30_Min[location]["info"]++

	} else {
		vars.Counters_For_Containers_Last_30_Min[location]["other"]++
	}

	db.Put([]byte(message_item[0]), []byte(message_item[1]), nil)
}

func CreateUser(login string, password string) error {
	if IsUserExists(login) {
		return errors.New("User is already exists")
	}

	vars.UsersDB.Put([]byte(login), []byte(password), nil)
	return nil
}

func GetUsers() []string {
	users := []string{}
	iter := vars.UsersDB.NewIterator(nil, nil)
	for iter.Next() {
		users = append(users, string(iter.Key()))
	}
	defer iter.Release()

	return users
}

func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool) [][]string {
	var db *leveldb.DB
	var err error
	db = vars.ActiveDBs[container]
	if db == nil {
		path := "leveldb/hosts/" + host + "/containers/" + container + "/logs"

		_, pathErr := os.Stat(path)
		if os.IsNotExist(pathErr) {
			return [][]string{}
		}

		db, err = leveldb.OpenFile(path, nil)
		if err != nil {
			db, _ = leveldb.RecoverFile(path, nil)
		}
		defer db.Close()
	}

	iter := db.NewIterator(nil, nil)
	// position := 0
	counter := 0
	to_return := [][]string{}

	iter.Last()
	if startWith != "" {
		if !iter.Seek([]byte(startWith)) {
			return to_return
		}
		if getPrev && !include {
			iter.Next()
		} else if !include {
			iter.Prev()
		}
	}

	for counter < limit {
		if len(iter.Key()) == 0 {
			if getPrev {
				iter.Next()
			} else {
				iter.Prev()
			}
			counter++
			continue
		}

		if !containStr(string(iter.Value()), message, caseSensetivity) {
			if getPrev {
				iter.Next()
			} else {
				iter.Prev()
			}
			continue
		}

		datetime := strings.Split(string(iter.Key()), " +")[0]
		to_return = append(
			to_return,
			[]string{
				datetime, string(iter.Value()),
			},
		)
		if getPrev {
			iter.Next()
		} else {
			iter.Prev()
		}
		counter++
	}

	defer iter.Release()
	return to_return
}

func EditUser(login string, password string) {
	vars.UsersDB.Put([]byte(login), []byte(password), nil)
}

func DeleteUnusedTokens() {
	for {
		db, r := leveldb.OpenFile("leveldb/tokens", nil)
		if r != nil {
			panic(r)
		}

		iter := db.NewIterator(nil, nil)
		for iter.Next() {
			wasUsed := string(iter.Value())
			if wasUsed == "was used" {
				continue
			}

			created, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", string(wasUsed))
			if created.Before(time.Now()) {
				db.Delete(iter.Key(), nil)
			}
		}
		iter.Release()
		db.Close()
		time.Sleep(time.Hour * 1)
	}
}

func DeleteContainer(host string, container string) {
	if vars.ActiveDBs[container] != nil {
		vars.ActiveDBs[container].Close()
	}
	os.RemoveAll("leveldb/hosts" + host + "/container/" + container)
}

// TODO remove all logs
func DeleteContainerLogs(host string, container string) {
	path := "leveldb/hosts" + host + "/container/" + container + "/logs"
	files, _ := os.ReadDir(path)
	lastNum := 0
	var lastName string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".log") {
			os.Remove(path + "/" + file.Name())
			continue
		}

		if !strings.HasSuffix(file.Name(), "ldb") {
			continue
		}

		num, _ := strconv.Atoi(file.Name()[:len(file.Name())-4])
		if num > lastNum {
			lastNum = num
			lastName = file.Name()
		}
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), "ldb") || file.Name() == lastName {
			continue
		}

		os.Remove(path + "/" + file.Name())
	}
}

func DeleteUser(login string, password string) error {
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	if !isExists {
		return errors.New("No such user")
	}

	vars.UsersDB.Delete([]byte(login), nil)
	return nil
}

func CheckUserPassword(login string, gotPassword string) bool {
	password, err := vars.UsersDB.Get([]byte(login), nil)
	if err != nil || strings.Compare(string(password), gotPassword) != 0 {
		return false
	}

	return true
}
