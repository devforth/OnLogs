package db

import (
	"fmt"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

func CreateOnLogsToken() string {
	token := util.GenerateJWTSecret()
	to_put := time.Now().UTC().Add(24 * time.Hour).String()
	err := vars.TokensDB.Put([]byte(token), []byte(to_put), nil)
	if err != nil {
		vars.TokensDB.Close()
		vars.TokensDB, vars.TokensDBErr = leveldb.OpenFile("leveldb/tokens", nil)

		err = vars.TokensDB.Put([]byte(token), []byte(to_put), nil)
		if err != nil {
			panic(err)
		}
	}
	fmt.Println("created token " + token)
	return token
}

func IsTokenExists(token string) bool {
	iter := vars.TokensDB.NewIterator(nil, nil)
	defer iter.Release()
	iter.First()
	if string(iter.Key()) == token {
		vars.TokensDB.Put([]byte(token), []byte("was used"), nil)
		return true
	}
	for iter.Next() {
		if string(iter.Key()) == token {
			vars.TokensDB.Put([]byte(token), []byte("was used"), nil)
			return true
		}
	}
	return false
}

func DeleteUnusedTokens() {
	for {
		db := vars.TokensDB
		iter := db.NewIterator(nil, nil)
		for iter.Next() {
			wasUsed := string(iter.Value())
			if wasUsed == "was used" {
				continue
			}

			created, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", string(wasUsed))
			if created.Before(time.Now()) {
				db.Delete(iter.Key(), nil)
			}
		}
		iter.Release()
		db.Close()
		time.Sleep(time.Hour * 1)
	}
}
