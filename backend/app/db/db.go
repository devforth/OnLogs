package db

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/devforth/OnLogs/app/util"
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

func GetLogs(host string, container string, message string, limit int, offset int, startWith string, caseSensetivity bool) [][]string {
	var db *leveldb.DB
	var err error
	var path string
	db = vars.ActiveDBs[container]
	if db == nil {
		if host == util.GetHost() {
			path = "leveldb/logs/" + container
		} else {
			path = "leveldb/hosts/" + host + "/" + container
		}

		db, err = leveldb.OpenFile(path, nil)
		if err != nil {
			db, _ = leveldb.RecoverFile(path, nil)
		}
		defer db.Close()
	}

	iter := db.NewIterator(nil, nil)
	position := 0
	counter := 0
	to_return := [][]string{}

	iter.Last()
	if startWith == "" {
		for position < offset {
			iter.Prev()
			if containStr(string(iter.Value()), message, caseSensetivity) {
				position++
			}
			if len(iter.Key()) == 0 {
				return to_return
			}
		}
	} else {
		if !iter.Seek([]byte(startWith)) {
			return to_return
		}
	}

	for counter < limit {
		if len(iter.Key()) == 0 {
			iter.Prev()
			counter++
			continue
		}

		if !containStr(string(iter.Value()), message, caseSensetivity) {
			iter.Prev()
			continue
		}

		datetime := strings.Split(string(iter.Key()), " +")[0]
		to_return = append(
			to_return,
			[]string{
				datetime, string(iter.Value()),
			},
		)
		iter.Prev()
		counter++
	}

	defer iter.Release()
	return to_return
}

func EditUser(login string, password string) {
	vars.UsersDB.Put([]byte(login), []byte(password), nil)
}

func DeleteContainer(host string, container string) {
	var path string
	if host == util.GetHost() {
		db := vars.ActiveDBs[container]
		if db != nil {
			db.Close()
		}
		path = "leveldb/logs/" + container
	} else {
		path = "leveldb/" + host + "/" + container
	}

	os.RemoveAll(path)
}

func DeleteContainerLogs(host string, container string) {
	var path string
	if host == util.GetHost() {
		path = "leveldb/logs/" + container
	} else {
		path = "leveldb/" + host + "/" + container
	}

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
