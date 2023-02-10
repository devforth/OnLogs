package userdb

import (
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestIsUserExists(t *testing.T) {
	if IsUserExists("random") {
		t.Error("User shouldn't exist")
	}
}

func TestCreateUser(t *testing.T) {
	CreateUser("amogus", "sus")
	if !IsUserExists("amogus") {
		t.Error("User is not exist")
	}

	err := CreateUser("amogus", "sus")
	if err == nil || err.Error() != "User is already exists" {
		t.Error("Error \"User is already exists\" expected, got ", err)
	}
}

func TestGetUsers(t *testing.T) {
	CreateUser("admin", "admin")
	CreateUser("admin1", "admin")
	CreateUser("admin2", "admin")

	users := GetUsers()
	if len(users) < 3 {
		t.Error("Need more users")
	}
}

func TestEditUser(t *testing.T) {
	CreateUser("testtest", "testtest")
	EditUser("testtest", "sus?")

	pass, _ := vars.UsersDB.Get([]byte("testtest"), nil)
	if string(pass) != "sus?" {
		t.Error("User wasn't edited")
	}
}

func TestDeleteUser(t *testing.T) {
	err := DeleteUser("aaaaaaaaa????????", "123")
	if err == nil {
		t.Error("Error is nil")
	}

	DeleteUser("admin1", "admin")
	if IsUserExists("admin1") {
		t.Error("User should be deleted")
	}
}

func TestCheckUserPassword(t *testing.T) {
	CreateUser("testpassword", "123456")
	if !CheckUserPassword("testpassword", "123456") {
		t.Error("Password should be correct")
	}
	if CheckUserPassword("testpassword", "1") {
		t.Error("Password shouldn't be correct")
	}
}
