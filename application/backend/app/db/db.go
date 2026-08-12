package db

import (
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	tokenTTL      = 24 * time.Hour
	legacyClaimed = "was used"
)

// Stored as "<expiry>|<claimedAt>"; an empty claimedAt means never claimed.
type tokenRecord struct {
	expiry    time.Time
	claimedAt time.Time
	claimed   bool
}

func encodeToken(r tokenRecord) string {
	claimed := ""
	if r.claimed {
		claimed = r.claimedAt.UTC().Format(time.RFC3339Nano)
	}
	return r.expiry.UTC().Format(time.RFC3339Nano) + "|" + claimed
}

func decodeToken(raw string) (tokenRecord, bool) {
	// Tokens issued before the expiry was tracked carry this literal and have no
	// recoverable timestamps; they stay valid so agents survive the upgrade.
	if raw == legacyClaimed {
		return tokenRecord{claimed: true}, true
	}

	expiryStr, claimedStr, ok := strings.Cut(raw, "|")
	if !ok {
		return tokenRecord{}, false
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiryStr)
	if err != nil {
		return tokenRecord{}, false
	}
	r := tokenRecord{expiry: expiry}
	if claimedStr != "" {
		claimedAt, err := time.Parse(time.RFC3339Nano, claimedStr)
		if err != nil {
			return tokenRecord{}, false
		}
		r.claimedAt = claimedAt
		r.claimed = true
	}
	return r, true
}

func CreateOnLogsToken() string {
	token := util.GenerateJWTSecret()
	to_put := encodeToken(tokenRecord{expiry: time.Now().UTC().Add(tokenTTL)})

	err := vars.TokensDB.Put([]byte(token), []byte(to_put), nil)
	if err != nil {
		vars.TokensDB.Close()
		vars.TokensDB, vars.TokensDBErr = leveldb.OpenFile("leveldb/tokens", nil)

		err = vars.TokensDB.Put([]byte(token), []byte(to_put), nil)
		if err != nil {
			panic(err)
		}
	}
	return token
}

func IsTokenExists(token string) bool {
	if token == "" || vars.TokensDB == nil {
		return false
	}

	raw, err := vars.TokensDB.Get([]byte(token), nil)
	if err != nil {
		return false
	}

	record, ok := decodeToken(string(raw))
	if !ok {
		return false
	}
	if record.claimed {
		return true
	}
	if record.expiry.Before(time.Now()) {
		return false
	}

	record.claimedAt = time.Now()
	record.claimed = true
	vars.TokensDB.Put([]byte(token), []byte(encodeToken(record)), nil)
	return true
}

func reapExpiredTokens(db *leveldb.DB) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		record, ok := decodeToken(string(iter.Value()))
		if !ok || (!record.claimed && record.expiry.Before(time.Now())) {
			db.Delete(iter.Key(), nil)
		}
	}
}

func DeleteUnusedTokens() {
	for {
		time.Sleep(time.Hour * 1)
		if vars.TokensDB != nil {
			reapExpiredTokens(vars.TokensDB)
		}
	}
}
