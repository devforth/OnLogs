package userdb

import (
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestCheckUserPasswordRejectsAnEmptyPassword(t *testing.T) {
	if err := vars.UsersDB.Put([]byte("emptypwuser"), []byte(""), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.UsersDB.Delete([]byte("emptypwuser"), nil) })

	if CheckUserPassword("emptypwuser", "") {
		t.Fatal("a user stored with an empty password authenticated with an empty password")
	}
}

func TestCheckUserPasswordRejectsUnknownUser(t *testing.T) {
	if CheckUserPassword("no-such-user-at-all", "") {
		t.Fatal("unknown user authenticated with an empty password")
	}
	if CheckUserPassword("no-such-user-at-all", "anything") {
		t.Fatal("unknown user authenticated")
	}
}

func TestStoredPasswordsAreNotPlaintext(t *testing.T) {
	const secret = "correct horse battery staple"
	CreateUser("hasheduser", secret)
	t.Cleanup(func() { vars.UsersDB.Delete([]byte("hasheduser"), nil) })

	stored, err := vars.UsersDB.Get([]byte("hasheduser"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), secret) {
		t.Fatalf("the password is stored in cleartext: %q", stored)
	}
	if !CheckUserPassword("hasheduser", secret) {
		t.Fatal("the correct password no longer verifies")
	}
	if CheckUserPassword("hasheduser", secret+"x") {
		t.Fatal("a wrong password verifies")
	}
}

func TestEditUserStoresAHash(t *testing.T) {
	CreateUser("edithashuser", "first-password")
	t.Cleanup(func() { vars.UsersDB.Delete([]byte("edithashuser"), nil) })

	EditUser("edithashuser", "second-password")

	stored, _ := vars.UsersDB.Get([]byte("edithashuser"), nil)
	if strings.Contains(string(stored), "second-password") {
		t.Fatalf("EditUser stored the password in cleartext: %q", stored)
	}
	if !CheckUserPassword("edithashuser", "second-password") {
		t.Fatal("the new password does not verify")
	}
	if CheckUserPassword("edithashuser", "first-password") {
		t.Fatal("the old password still verifies")
	}
}

func TestLegacyPlaintextPasswordVerifiesAndIsUpgraded(t *testing.T) {
	if err := vars.UsersDB.Put([]byte("legacyuser"), []byte("legacy-secret"), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.UsersDB.Delete([]byte("legacyuser"), nil) })

	if !CheckUserPassword("legacyuser", "legacy-secret") {
		t.Fatal("an account created before hashing stopped working")
	}

	stored, _ := vars.UsersDB.Get([]byte("legacyuser"), nil)
	if string(stored) == "legacy-secret" {
		t.Fatal("a verified legacy password was not upgraded to a hash")
	}
	if !CheckUserPassword("legacyuser", "legacy-secret") {
		t.Fatal("the password stopped verifying after the upgrade")
	}
}
