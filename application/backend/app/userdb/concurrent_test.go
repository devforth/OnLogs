package userdb

import (
	"sync/atomic"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func pauseFirstHash(t *testing.T) (entered <-chan struct{}, resume func()) {
	in, out := make(chan struct{}), make(chan struct{})
	var taken atomic.Bool
	hashPassword = func(password string) string {
		if taken.CompareAndSwap(false, true) {
			close(in)
			<-out
		}
		return HashPassword(password)
	}
	t.Cleanup(func() { hashPassword = HashPassword })
	return in, func() { close(out) }
}

func resetUser(t *testing.T, login string) {
	vars.UsersDB.Delete([]byte(login), nil)
	t.Cleanup(func() { vars.UsersDB.Delete([]byte(login), nil) })
}

func TestEditDuringDeleteDoesNotRecreateTheUser(t *testing.T) {
	const login = "race-edit-delete"
	resetUser(t, login)
	if err := CreateUser(login, "pw"); err != nil {
		t.Fatal(err)
	}

	entered, resume := pauseFirstHash(t)
	done := make(chan error)
	go func() { done <- EditUser(login, "edited-pw") }()
	<-entered
	if err := DeleteUser(login); err != nil {
		t.Fatal(err)
	}
	resume()

	if err := <-done; err == nil || err.Error() != "No such user" {
		t.Fatalf("edit after a delete returned %v, want No such user", err)
	}
	if IsUserExists(login) {
		t.Fatal("the deleted user was re-created by a concurrent edit")
	}
}

func TestConcurrentCreatesOfOneLoginLetOnlyOneSucceed(t *testing.T) {
	const login = "race-double-create"
	resetUser(t, login)

	entered, resume := pauseFirstHash(t)
	done := make(chan error)
	go func() { done <- CreateUser(login, "alpha-pw") }()
	<-entered
	if err := CreateUser(login, "bravo-pw"); err != nil {
		t.Fatal(err)
	}
	resume()

	if err := <-done; err == nil || err.Error() != "User is already exists" {
		t.Fatalf("second create returned %v, want User is already exists", err)
	}
	if !CheckUserPassword(login, "bravo-pw") || CheckUserPassword(login, "alpha-pw") {
		t.Fatal("the acknowledged password does not log in")
	}
}

func TestLegacyLoginUpgradeKeepsAConcurrentPasswordChange(t *testing.T) {
	const login = "race-legacy-edit"
	resetUser(t, login)
	if err := vars.UsersDB.Put([]byte(login), []byte("old-password"), nil); err != nil {
		t.Fatal(err)
	}

	entered, resume := pauseFirstHash(t)
	done := make(chan bool)
	go func() { done <- CheckUserPassword(login, "old-password") }()
	<-entered
	if err := EditUser(login, "new-password"); err != nil {
		t.Fatal(err)
	}
	resume()
	<-done

	if CheckUserPassword(login, "old-password") || !CheckUserPassword(login, "new-password") {
		t.Fatal("a legacy login upgrade reverted the password change")
	}
}

func TestLegacyLoginUpgradeDoesNotRecreateADeletedUser(t *testing.T) {
	const login = "race-legacy-delete"
	resetUser(t, login)
	if err := vars.UsersDB.Put([]byte(login), []byte("old-password"), nil); err != nil {
		t.Fatal(err)
	}

	entered, resume := pauseFirstHash(t)
	done := make(chan bool)
	go func() { done <- CheckUserPassword(login, "old-password") }()
	<-entered
	if err := DeleteUser(login); err != nil {
		t.Fatal(err)
	}
	resume()
	<-done

	if IsUserExists(login) {
		t.Fatal("a legacy login upgrade re-created the deleted user")
	}
}
