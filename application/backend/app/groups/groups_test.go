package groups

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateGroupName(t *testing.T) {
	cases := []struct {
		label   string
		name    string
		wantErr bool
	}{
		{"plain name", "backend", false},
		{"name with a space", "backend services", false},
		{"exactly 64 bytes", strings.Repeat("a", 64), false},
		{"multi-byte name inside the byte limit", strings.Repeat("é", 32), false},
		{"empty", "", true},
		{"65 bytes", strings.Repeat("a", 65), true},
		{"multi-byte name over the byte limit", strings.Repeat("é", 33), true},
		{"NUL byte", "web\x00admin", true},
		{"NUL byte at the end", "web\x00", true},
		{"leading whitespace", " backend", true},
		{"trailing whitespace", "backend ", true},
		{"trailing newline", "backend\n", true},
		{"only whitespace", "   ", true},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			err := ValidateGroupName(c.name)
			if c.wantErr && err == nil {
				t.Fatalf("ValidateGroupName(%q) = nil, want an error", c.name)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("ValidateGroupName(%q) = %v, want nil", c.name, err)
			}
		})
	}
}

func TestValidateMembers(t *testing.T) {
	cases := []struct {
		label   string
		members []Member
		wantErr bool
	}{
		{"empty list", []Member{}, false},
		{"one member", []Member{{Host: "host1", Service: "api"}}, false},
		{"exactly 500 members", makeMembers(500), false},
		{"501 members", makeMembers(501), true},
		{"empty host", []Member{{Host: "", Service: "api"}}, true},
		{"empty service", []Member{{Host: "host1", Service: ""}}, true},
		{"host is a parent traversal", []Member{{Host: "..", Service: "api"}}, true},
		{"service is a parent traversal", []Member{{Host: "host1", Service: ".."}}, true},
		{"host is a dot", []Member{{Host: ".", Service: "api"}}, true},
		{"service has a path separator", []Member{{Host: "host1", Service: "a/b"}}, true},
		{"host has a path separator", []Member{{Host: "a/b", Service: "api"}}, true},
		{"service has a backslash", []Member{{Host: "host1", Service: `a\b`}}, true},
		{"service has a NUL byte", []Member{{Host: "host1", Service: "a\x00b"}}, true},
		{"one bad member among good ones", []Member{
			{Host: "host1", Service: "api"},
			{Host: "host1", Service: ".."},
			{Host: "host2", Service: "web"},
		}, true},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			err := ValidateMembers(c.members)
			if c.wantErr && err == nil {
				t.Fatalf("ValidateMembers(%v) = nil, want an error", c.members)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("ValidateMembers(%v) = %v, want nil", c.members, err)
			}
		})
	}
}

// The key separator is what keeps one user's groups out of another's, so a name
// carrying its own separator must never be storable.
func TestKeySeparatorCannotBeForged(t *testing.T) {
	if err := ValidateGroupName("\x00victim's group"); err == nil {
		t.Fatal("a group name containing the key separator was accepted")
	}

	forged, err := Key("attacker", "\x00victim\x00group")
	if err == nil {
		t.Fatalf("Key accepted a name containing the separator: %q", forged)
	}

	mine, err := Key("attacker", "mine")
	if err != nil {
		t.Fatalf("Key rejected a valid name: %v", err)
	}
	if !bytes.Equal(mine, []byte("attacker\x00mine")) {
		t.Fatalf("Key(attacker, mine) = %q, want %q", mine, "attacker\x00mine")
	}

	if _, err := Key("attac\x00ker", "mine"); err == nil {
		t.Fatal("Key accepted a username containing the separator")
	}
}

func TestStoreLoadDeleteRoundTrip(t *testing.T) {
	members := []Member{{Host: "host1", Service: "api"}, {Host: "host2", Service: "web"}}
	if err := Store("alice", "backend", members); err != nil {
		t.Fatalf("Store: %v", err)
	}
	t.Cleanup(func() { Delete("alice", "backend") })

	loaded, found, err := Load("alice", "backend")
	if err != nil || !found {
		t.Fatalf("Load: found=%v err=%v", found, err)
	}
	if len(loaded) != 2 || loaded[0] != members[0] || loaded[1] != members[1] {
		t.Fatalf("Load returned %v, want %v", loaded, members)
	}

	if err := Delete("alice", "backend"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, found, _ := Load("alice", "backend"); found {
		t.Fatal("the group is still readable after Delete")
	}
	// The UI can double-fire delete, so a second one is not an error.
	if err := Delete("alice", "backend"); err != nil {
		t.Fatalf("second Delete: %v", err)
	}
}

// List keys off a username prefix, so a username that is a prefix of another
// must not pull in the longer user's groups.
func TestListIsScopedToOneUser(t *testing.T) {
	Store("bob", "shared", []Member{{Host: "host1", Service: "api"}})
	Store("bobby", "other", []Member{{Host: "host1", Service: "web"}})
	t.Cleanup(func() {
		Delete("bob", "shared")
		Delete("bobby", "other")
	})

	bobs, err := List("bob")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(bobs) != 1 || bobs[0].Name != "shared" {
		t.Fatalf("List(bob) = %v, want just the group named shared", bobs)
	}
	if _, found, _ := Load("bob", "other"); found {
		t.Fatal("bob can read bobby's group")
	}
}

func TestStoreRejectsInvalidInput(t *testing.T) {
	if err := Store("carol", "bad\x00name", nil); err == nil {
		t.Error("Store accepted a name containing the key separator")
	}
	if err := Store("carol", "traversal", []Member{{Host: "..", Service: "api"}}); err == nil {
		t.Error("Store accepted a member that escapes its directory")
	}
}

func makeMembers(n int) []Member {
	members := make([]Member, 0, n)
	for i := 0; i < n; i++ {
		members = append(members, Member{Host: "host1", Service: "svc" + strings.Repeat("x", i%5)})
	}
	return members
}
