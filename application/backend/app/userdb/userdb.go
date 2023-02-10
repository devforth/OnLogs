package userdb

import (
	"errors"
	"strings"

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
