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
	iter.Release()

	return users
}

func GetLogs(container string, message string, limit int, offset int) []string {

	iter := vars.DB.NewIterator(nil, nil)
	iter.Last()
	position := 0
	counter := 0
	for position < offset {
		iter.Prev()
		position++
	}

	to_return := []string{}
	for counter < limit {
		if len(iter.Key()) == 0 {
			iter.Prev()
			counter++
			continue
		}

		datetime := strings.Split(string(iter.Key()), " +")[0]
		if len(datetime) < 29 {
			datetime = datetime + strings.Repeat("0", 29-len(datetime))
		}
		to_return = append(to_return, datetime+" "+string(iter.Value()))
		iter.Prev()
		counter++
	}
	iter.Release()
	// container_db.Close()
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
