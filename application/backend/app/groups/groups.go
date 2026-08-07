// Package groups stores per-user, named sets of host/service pairs. It owns the
// key layout and the JSON encoding so no caller has to reproduce either.
package groups

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	ldbutil "github.com/syndtr/goleveldb/leveldb/util"
)

const (
	MaxNameBytes     = 64
	MaxMembers       = 500
	MaxGroupsPerUser = 100

	// The only byte that cannot appear in a group name, which is what keeps one
	// user's key range out of another's.
	separator = "\x00"
)

type Member struct {
	Host    string `json:"host"`
	Service string `json:"service"`
}

type Group struct {
	Name    string   `json:"name"`
	Members []Member `json:"members"`
}

func ValidateGroupName(name string) error {
	if name == "" {
		return errors.New("Group name can not be empty")
	}
	if len(name) > MaxNameBytes {
		return fmt.Errorf("Group name can not be longer than %d bytes", MaxNameBytes)
	}
	if strings.ContainsRune(name, 0) {
		return errors.New("Group name can not contain a NUL byte")
	}
	if strings.TrimSpace(name) != name {
		return errors.New("Group name can not start or end with whitespace")
	}
	return nil
}

// Every member reaches util.GetDB and a /view/:host/:service URL, so both halves
// have to survive the same single-path-component guard those already apply.
func ValidateMembers(members []Member) error {
	if len(members) > MaxMembers {
		return fmt.Errorf("A group can not hold more than %d members", MaxMembers)
	}
	for _, member := range members {
		if !util.IsSafeName(member.Host) || !util.IsSafeName(member.Service) {
			return errors.New("Invalid group member")
		}
	}
	return nil
}

func Key(username string, name string) ([]byte, error) {
	if strings.ContainsRune(username, 0) {
		return nil, errors.New("Invalid username")
	}
	if err := ValidateGroupName(name); err != nil {
		return nil, err
	}
	return []byte(username + separator + name), nil
}

func Load(username string, name string) ([]Member, bool, error) {
	key, err := Key(username, name)
	if err != nil {
		return nil, false, err
	}

	raw, err := vars.GroupsDB.Get(key, nil)
	if errors.Is(err, leveldb.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	members := []Member{}
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, false, err
	}
	return members, true, nil
}

func Store(username string, name string, members []Member) error {
	key, err := Key(username, name)
	if err != nil {
		return err
	}
	if err := ValidateMembers(members); err != nil {
		return err
	}

	if members == nil {
		members = []Member{}
	}
	raw, err := json.Marshal(members)
	if err != nil {
		return err
	}
	return vars.GroupsDB.Put(key, raw, nil)
}

// Deleting a key that is not there is not an error, which is what makes the
// route idempotent.
func Delete(username string, name string) error {
	key, err := Key(username, name)
	if err != nil {
		return err
	}
	return vars.GroupsDB.Delete(key, nil)
}

func List(username string) ([]Group, error) {
	if strings.ContainsRune(username, 0) {
		return nil, errors.New("Invalid username")
	}

	prefix := username + separator
	iter := vars.GroupsDB.NewIterator(ldbutil.BytesPrefix([]byte(prefix)), nil)
	defer iter.Release()

	list := []Group{}
	for iter.Next() {
		members := []Member{}
		if err := json.Unmarshal(iter.Value(), &members); err != nil {
			continue
		}
		list = append(list, Group{
			Name:    strings.TrimPrefix(string(iter.Key()), prefix),
			Members: members,
		})
	}
	return list, iter.Error()
}
