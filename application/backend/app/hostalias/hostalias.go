package hostalias

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

const MaxAliasBytes = 64

func Validate(host string, alias string) error {
	if !util.IsSafeName(host) {
		return errors.New("Invalid host")
	}
	if len(alias) > MaxAliasBytes {
		return fmt.Errorf("Display name can not be longer than %d bytes", MaxAliasBytes)
	}
	if strings.ContainsRune(alias, 0) {
		return errors.New("Display name can not contain a NUL byte")
	}
	if strings.TrimSpace(alias) != alias {
		return errors.New("Display name can not start or end with whitespace")
	}
	return nil
}

// An empty alias clears it, so the sidebar falls back to the real hostname.
func Set(host string, alias string) error {
	if err := Validate(host, alias); err != nil {
		return err
	}
	if alias == "" {
		return vars.AliasDB.Delete([]byte(host), nil)
	}
	return vars.AliasDB.Put([]byte(host), []byte(alias), nil)
}

func All() (map[string]string, error) {
	iter := vars.AliasDB.NewIterator(nil, nil)
	defer iter.Release()

	aliases := map[string]string{}
	for iter.Next() {
		aliases[string(iter.Key())] = string(iter.Value())
	}
	return aliases, iter.Error()
}
