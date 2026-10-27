package userdb

import (
	"testing"
	"time"

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

	if !CheckUserPassword("testtest", "sus?") {
		t.Error("User wasn't edited")
	}
	pass, _ := vars.UsersDB.Get([]byte("testtest"), nil)
	if string(pass) == "sus?" {
		t.Error("Password is stored in cleartext")
	}
}

func TestDeleteUser(t *testing.T) {
	err := DeleteUser("aaaaaaaaa????????")
	if err == nil {
		t.Error("Error is nil")
	}

	DeleteUser("admin1")
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

func TestEndedSessionsArePrunedOnceTheyExpire(t *testing.T) {
	if err := EndSession("lapsed-session", time.Now().Add(-time.Minute).Unix()); err != nil {
		t.Fatal(err)
	}
	if err := EndSession("live-session", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.EndedSessionsDB.Delete([]byte("live-session"), nil) })

	if ended, _ := IsSessionEnded("lapsed-session"); ended {
		t.Error("an entry past its cookie's expiry was kept")
	}
	if ended, _ := IsSessionEnded("live-session"); !ended {
		t.Error("an entry for an unexpired cookie was dropped")
	}
}
