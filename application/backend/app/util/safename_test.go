package util

import (
	"os"
	"testing"
)

func TestGetDBRefusesNamesThatLeaveTheTree(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := [][3]string{
		{"../../../PWN", "c", "logs"},
		{"realhost", "../../../../PWN", "logs"},
		{"realhost", "c", "../../../PWN"},
		{"", "c", "logs"},
		{"realhost", "", "logs"},
		{".", "c", "logs"},
		{"..", "c", "logs"},
		{"a/b", "c", "logs"},
	}

	for _, c := range cases {
		if db := GetDB(c[0], c[1], c[2]); db != nil {
			t.Errorf("GetDB(%q, %q, %q) opened a database", c[0], c[1], c[2])
		}
	}

	if _, err := os.Stat("PWN"); err == nil {
		t.Fatal("GetDB created a directory outside leveldb/hosts")
	}
	if entries, err := os.ReadDir("."); err == nil {
		for _, e := range entries {
			if e.Name() != "leveldb" {
				t.Errorf("unexpected entry created outside the tree: %s", e.Name())
			}
		}
	}
}

func TestGetDBStillOpensLegitimateNames(t *testing.T) {
	t.Chdir(t.TempDir())

	db := GetDB("somehost", "somecontainer", "logs")
	if db == nil {
		t.Fatal("GetDB refused a legitimate host/container/dbType")
	}
	db.Close()

	if _, err := os.Stat("leveldb/hosts/somehost/containers/somecontainer/logs"); err != nil {
		t.Fatalf("database was not created where expected: %v", err)
	}
}

func TestGetDirSizeRefusesNamesThatLeaveTheTree(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/realhost/containers", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("outside", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("outside/big", make([]byte, 512*1024), 0o600); err != nil {
		t.Fatal(err)
	}

	if size := GetDirSize("realhost", "../../../../outside"); size != 0 {
		t.Fatalf("GetDirSize measured a directory outside the tree: %v MiB", size)
	}
}

func TestGetDockerContainerIDRefusesNamesThatLeaveTheTree(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts", 0o700); err != nil {
		t.Fatal(err)
	}

	id := GetDockerContainerID("../..", "anything")
	if id != "" {
		t.Errorf("GetDockerContainerID accepted a traversing host: %q", id)
	}
	if _, err := os.Stat("containersMeta"); err == nil {
		t.Fatal("GetDockerContainerID created a LevelDB outside leveldb/hosts")
	}
}
