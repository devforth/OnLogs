package userdb

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

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
		if string(iter.Key()) == os.Getenv("ADMIN_USERNAME") {
			continue
		}
		users = append(users, string(iter.Key()))
	}
	defer iter.Release()

	return users
}

func EditUser(login string, password string) {
	vars.UsersDB.Put([]byte(login), []byte(password), nil)
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

func GetUserSettings(username string) map[string]interface{} {
	settingsDB, _ := leveldb.OpenFile("leveldb/usersSettings", nil)
	defer settingsDB.Close()
	var to_return map[string]interface{}
	result, _ := settingsDB.Get([]byte(username), nil)
	json.Unmarshal(result, &to_return)
	return to_return
}

func UpdateUserSettings(username string, settings map[string]interface{}) {
	settingsDB, _ := leveldb.OpenFile("leveldb/usersSettings", nil)
	defer settingsDB.Close()
	to_put, _ := json.Marshal(settings)
	settingsDB.Put([]byte(username), to_put, nil)
}
