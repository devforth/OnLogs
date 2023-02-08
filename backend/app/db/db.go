package db

import (
	"math/rand"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
)

func CreateOnLogsToken() string {
	tokenLen := 25
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)
	defer tokensDB.Close()

	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890!@#$%^&*()_+,.:'{}[]"
	b := make([]byte, tokenLen)

	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for i := range b {
		b[i] = letterBytes[r1.Int63()%int64(len(letterBytes))]
	}
	token := string(b)

	to_put := time.Now().UTC().Add(24 * time.Hour).String()
	tokensDB.Put(b, []byte(to_put), nil)
	return token
}

func IsTokenExists(token string) bool {
	tokensDB, _ := leveldb.OpenFile("leveldb/tokens", nil)

	iter := tokensDB.NewIterator(nil, nil)
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
	defer iter.Release()
	defer tokensDB.Close()

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
