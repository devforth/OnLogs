package db

import (
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
)

func TestIsTokenExistsKeepsTheTokenAuditable(t *testing.T) {
	token := CreateOnLogsToken()

	before, err := vars.TokensDB.Get([]byte(token), nil)
	if err != nil {
		t.Fatal(err)
	}
	issued := strings.Split(string(before), "|")[0]
	if _, err := time.Parse(time.RFC3339Nano, issued); err != nil {
		t.Fatalf("a fresh token does not carry a parseable expiry: %q", before)
	}

	if !IsTokenExists(token) {
		t.Fatal("a freshly minted token was rejected")
	}

	after, err := vars.TokensDB.Get([]byte(token), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(after), issued+"|") {
		t.Fatalf("using a token destroyed its expiry: %q -> %q", before, after)
	}
	if strings.HasSuffix(string(after), "|") {
		t.Fatalf("using a token did not record when it was claimed: %q", after)
	}
}

func TestIsTokenExistsWritesOnlyOnFirstUse(t *testing.T) {
	token := CreateOnLogsToken()

	if !IsTokenExists(token) {
		t.Fatal("a freshly minted token was rejected")
	}
	first, _ := vars.TokensDB.Get([]byte(token), nil)

	time.Sleep(5 * time.Millisecond)
	if !IsTokenExists(token) {
		t.Fatal("a claimed token was rejected")
	}
	second, _ := vars.TokensDB.Get([]byte(token), nil)

	if string(first) != string(second) {
		t.Fatalf("every ingested log line rewrites the token record: %q -> %q", first, second)
	}
}

func TestIsTokenExistsRejectsTheEmptyToken(t *testing.T) {
	if IsTokenExists("") {
		t.Fatal("the empty token authenticates")
	}
}

func TestIsTokenExistsRejectsAnUnclaimedExpiredToken(t *testing.T) {
	expired := "expired-token-probe"
	stamp := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano) + "|"
	if err := vars.TokensDB.Put([]byte(expired), []byte(stamp), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.TokensDB.Delete([]byte(expired), nil) })

	if IsTokenExists(expired) {
		t.Fatalf("an unclaimed token that expired an hour ago still authenticates (%q)", stamp)
	}
}

func TestIsTokenExistsRejectsAGarbageRecord(t *testing.T) {
	junk := "garbage-token-probe"
	if err := vars.TokensDB.Put([]byte(junk), []byte("not a timestamp at all"), nil); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vars.TokensDB.Delete([]byte(junk), nil) })

	if IsTokenExists(junk) {
		t.Fatal("a token record with no recoverable expiry still authenticates")
	}
}

func TestDeleteUnusedTokensKeepsTheSharedHandleOpen(t *testing.T) {
	token := CreateOnLogsToken()

	go DeleteUnusedTokens()
	time.Sleep(250 * time.Millisecond)

	if !IsTokenExists(token) {
		t.Error("a valid token stopped being accepted once the token reaper ran")
	}
	if IsTokenExists("") {
		t.Error("the empty token authenticates once the token reaper ran")
	}
}
