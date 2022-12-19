package db

import (
	"errors"
	"strings"

	vars "github.com/devforth/OnLogs/app/vars"
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

func GetLogs(container string, message string, limit int, offset int, startWith string) [][]string {
	db := vars.ActiveDBs[container]
	iter := db.NewIterator(nil, nil)
	position := 0
	counter := 0
	to_return := [][]string{}

	iter.Last()
	if startWith == "" {
		for position < offset {
			iter.Prev()
			if strings.Contains(string(iter.Value()), message) {
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

		if !strings.Contains(string(iter.Value()), message) {
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
