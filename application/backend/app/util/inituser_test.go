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
