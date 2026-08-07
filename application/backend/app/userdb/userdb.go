package userdb

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"

	"github.com/devforth/OnLogs/app/vars"
)

func IsUserExists(login string) bool {
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	return isExists
}

func CreateUser(login string, password string) error {
	if IsUserExists(login) {
		return errors.New("User is already exists")
	}

	return vars.UsersDB.Put([]byte(login), []byte(HashPassword(password)), nil)
}

func GetUsers() []map[string]interface{} {
	var users []map[string]interface{}
	iter := vars.UsersDB.NewIterator(nil, nil)
	for iter.Next() {
		var editable bool
		if string(iter.Key()) == os.Getenv("ADMIN_USERNAME") {
			editable = false
		} else {
			editable = true
		}

		user := map[string]interface{}{
			"username": string(iter.Key()),
			"editable": editable,
		}
		users = append(users, user)
	}
	defer iter.Release()

	return users
}

func EditUser(login string, password string) error {
	return vars.UsersDB.Put([]byte(login), []byte(HashPassword(password)), nil)
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
	// goleveldb returns ([]byte{}, nil) for a key stored with an empty value.
	if login == "" || gotPassword == "" {
		return false
	}

	stored, err := vars.UsersDB.Get([]byte(login), nil)
	if err != nil || len(stored) == 0 {
		return false
	}

	if ok, wasHashed := verifyHash(string(stored), gotPassword); wasHashed {
		return ok
	}

	// Accounts predating hashing hold the password verbatim; verify, then upgrade.
	if subtle.ConstantTimeCompare(stored, []byte(gotPassword)) != 1 {
		return false
	}
	vars.UsersDB.Put([]byte(login), []byte(HashPassword(gotPassword)), nil)
	return true
}

func GetUserSettings(username string) map[string]interface{} {
	var to_return map[string]interface{}
	vars.Mutex.Lock()
	result, _ := vars.SettingsDB.Get([]byte(username), nil)
	vars.Mutex.Unlock()
	json.Unmarshal(result, &to_return)
	return to_return
}

func UpdateUserSettings(username string, settings map[string]interface{}) {
	to_put, _ := json.Marshal(settings)
	vars.Mutex.Lock()
	vars.SettingsDB.Put([]byte(username), to_put, nil)
	vars.Mutex.Unlock()
}
