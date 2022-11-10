package db

import (
	"errors"
	"strings"

	vars "github.com/devforth/OnLogs/app/vars"
)

func CreateUser(login string, password string) error {
	isExists, _ := vars.UsersDB.Has([]byte(login), nil)
	if isExists {
		return errors.New("User is already exists")
	}

	vars.UsersDB.Put([]byte(login), []byte(password), nil)
	return nil
}

func CheckUserPassword(login string, gotPassword string) bool {
	password, err := vars.UsersDB.Get([]byte(login), nil)
	if err != nil || strings.Compare(string(password), gotPassword) != 0 {
		return false
	}

	return true
}
