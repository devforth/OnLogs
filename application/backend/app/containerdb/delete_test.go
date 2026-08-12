package containerdb

import (
	"os"
	"testing"
)

func TestDeleteContainerCannotRemoveATreeOutsideLeveldb(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/realhost/containers", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("victim_tree/nested", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("victim_tree/nested/important", []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	DeleteContainer("realhost", "../../../../victim_tree", true)

	if _, err := os.Stat("victim_tree/nested/important"); err != nil {
		t.Fatalf("an admin delete removed a tree outside leveldb/hosts: %v", err)
	}
}

func TestDeleteContainerLogsCannotEmptyATreeOutsideLeveldb(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/realhost/containers", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("victim_tree/nested", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("victim_tree/nested/important", []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	DeleteContainer("realhost", "../../../../victim_tree", false)

	if _, err := os.Stat("victim_tree/nested/important"); err != nil {
		t.Fatalf("a clear-logs request emptied a tree outside leveldb/hosts: %v", err)
	}
}
