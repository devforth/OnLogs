package db

import (
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/syndtr/goleveldb/leveldb"
)

func CreateOnLogsToken() string {
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)
	defer tokensDB.Close()

	token := util.GenerateJWTSecret()
	to_put := time.Now().UTC().Add(24 * time.Hour).String()
	tokensDB.Put([]byte(token), []byte(to_put), nil)
	return token
}

func IsTokenExists(token string) bool {
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)
	defer tokensDB.Close()

	iter := tokensDB.NewIterator(nil, nil)
	defer iter.Release()
	iter.First()
	if string(iter.Key()) == token {
		tokensDB.Put([]byte(token), []byte("was used"), nil)
		return true
	}
	for iter.Next() {
		if string(iter.Key()) == token {
			tokensDB.Put([]byte(token), []byte("was used"), nil)
			return true
		}
	}

	return false
}

func DeleteUnusedTokens() {
	for {
		db, r := leveldb.OpenFile("leveldb/tokens", nil)
		if r != nil {
			panic(r)
		}

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
