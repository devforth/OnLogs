package util

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestCreateInitUserRefusesAnEmptyAdminPassword(t *testing.T) {
	const probe = "admin-empty-pw-probe"
	os.Setenv("ADMIN_USERNAME", probe)
	os.Setenv("ADMIN_PASSWORD", "")
	t.Cleanup(func() {
		vars.UsersDB.Delete([]byte(probe), nil)
		os.Setenv("ADMIN_USERNAME", "admin")
		os.Setenv("ADMIN_PASSWORD", "")
	})

	CreateInitUser()

	exists, err := vars.UsersDB.Has([]byte(probe), nil)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		stored, _ := vars.UsersDB.Get([]byte(probe), nil)
		t.Fatalf("admin was created with an empty ADMIN_PASSWORD (stored value %q)", stored)
	}
}

func TestChangedAdminPasswordEndsAdminSessions(t *testing.T) {
	const probe = "admin-rotation-probe"
	os.Setenv("JWT_SECRET", "a-real-secret-value-for-the-test")
	os.Setenv("ADMIN_USERNAME", probe)
	os.Setenv("ADMIN_PASSWORD", "first-admin-password")
	t.Cleanup(func() {
		vars.UsersDB.Delete([]byte(probe), nil)
		os.Setenv("ADMIN_USERNAME", "admin")
		os.Setenv("ADMIN_PASSWORD", "")
	})
	if err := CreateInitUser(); err != nil {
		t.Fatal(err)
	}
	session := requestWithCookie(CreateJWT(probe))

	if err := CreateInitUser(); err != nil {
		t.Fatal(err)
	}
	if _, err := GetUserFromJWT(session); err != nil {
		t.Fatalf("a restart with the same ADMIN_PASSWORD ended the admin session: %v", err)
	}

	os.Setenv("ADMIN_PASSWORD", "rotated-admin-password")
	if err := CreateInitUser(); err != nil {
		t.Fatal(err)
	}
	if user, err := GetUserFromJWT(session); err == nil {
		t.Errorf("a session issued before the ADMIN_PASSWORD change still authenticates as %q", user)
	}
	if _, err := GetUserFromJWT(requestWithCookie(CreateJWT(probe))); err != nil {
		t.Errorf("a session issued after the ADMIN_PASSWORD change was rejected: %v", err)
	}
}
