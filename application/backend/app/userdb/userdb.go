package userdb

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/devforth/OnLogs/app/vars"
)

var hashPassword = HashPassword

// Guards check-then-write on the user store; hash before taking it.
var usersMu sync.Mutex

func IsUserExists(login string) bool {
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	return isExists
}

func CreateUser(login string, password string) error {
	if IsUserExists(login) {
		return errors.New("User is already exists")
	}
	hash := hashPassword(password)

	usersMu.Lock()
	defer usersMu.Unlock()
	if IsUserExists(login) {
		return errors.New("User is already exists")
	}
	return vars.UsersDB.Put([]byte(login), []byte(hash), nil)
}

func GetUsers() []map[string]interface{} {
	var users []map[string]interface{}
	iter := vars.UsersDB.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		users = append(users, map[string]interface{}{
			"username": string(iter.Key()),
			"editable": string(iter.Key()) != os.Getenv("ADMIN_USERNAME"),
		})
	}
	return users
}

func EditUser(login string, password string) error {
	hash := hashPassword(password)

	usersMu.Lock()
	defer usersMu.Unlock()
	if !IsUserExists(login) {
		return errors.New("No such user")
	}
	return vars.UsersDB.Put([]byte(login), []byte(hash), nil)
}

func DeleteUser(login string) error {
	usersMu.Lock()
	defer usersMu.Unlock()
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	if !isExists {
		return errors.New("No such user")
	}

	return vars.UsersDB.Delete([]byte(login), nil)
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
	hash := hashPassword(gotPassword)

	usersMu.Lock()
	defer usersMu.Unlock()
	if current, err := vars.UsersDB.Get([]byte(login), nil); err == nil && bytes.Equal(current, stored) {
		vars.UsersDB.Put([]byte(login), []byte(hash), nil)
	}
	return true
}

func GetUserSettings(username string) map[string]interface{} {
	var to_return map[string]interface{}
	if vars.SettingsDB == nil {
		return to_return
	}

	vars.Mutex.Lock()
	result, _ := vars.SettingsDB.Get([]byte(username), nil)
	vars.Mutex.Unlock()
	json.Unmarshal(result, &to_return)
	return to_return
}

func UpdateUserSettings(username string, settings map[string]interface{}) error {
	to_put, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	if vars.SettingsDB == nil {
		return errors.New("settings database is unavailable")
	}

	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()
	return vars.SettingsDB.Put([]byte(username), to_put, nil)
}
