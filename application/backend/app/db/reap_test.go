package db

import (
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
)

func TestReapExpiredTokensRemovesOnlyUnclaimedExpiredOnes(t *testing.T) {
	claimed := "reap-claimed-probe"
	unclaimedFresh := "reap-unclaimed-fresh-probe"
	unclaimedStale := "reap-unclaimed-stale-probe"
	garbage := "reap-garbage-probe"
	t.Cleanup(func() {
		for _, k := range []string{claimed, unclaimedFresh, unclaimedStale, garbage} {
			vars.TokensDB.Delete([]byte(k), nil)
		}
	})

	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	vars.TokensDB.Put([]byte(claimed), []byte(encodeToken(tokenRecord{expiry: past, claimedAt: past, claimed: true})), nil)
	vars.TokensDB.Put([]byte(unclaimedFresh), []byte(encodeToken(tokenRecord{expiry: future})), nil)
	vars.TokensDB.Put([]byte(unclaimedStale), []byte(encodeToken(tokenRecord{expiry: past})), nil)
	vars.TokensDB.Put([]byte(garbage), []byte("not a timestamp at all"), nil)

	reapExpiredTokens(vars.TokensDB)

	for _, keep := range []string{claimed, unclaimedFresh} {
		if has, _ := vars.TokensDB.Has([]byte(keep), nil); !has {
			t.Errorf("%s was reaped but should have been kept", keep)
		}
	}
	for _, gone := range []string{unclaimedStale, garbage} {
		if has, _ := vars.TokensDB.Has([]byte(gone), nil); has {
			t.Errorf("%s survived the reaper", gone)
		}
	}
}

func TestLegacyClaimedTokensKeepWorking(t *testing.T) {
	legacy := "legacy-was-used-probe"
	if err := vars.TokensDB.Put([]byte(legacy), []byte(legacyClaimed), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.TokensDB.Delete([]byte(legacy), nil) })

	if !IsTokenExists(legacy) {
		t.Fatal("an agent token issued before the upgrade stopped working")
	}
	reapExpiredTokens(vars.TokensDB)
	if has, _ := vars.TokensDB.Has([]byte(legacy), nil); !has {
		t.Fatal("an agent token issued before the upgrade was reaped")
	}
}
