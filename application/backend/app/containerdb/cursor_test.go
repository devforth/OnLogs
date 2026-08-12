package containerdb

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
)

const sameMillisecond = "-02-10T12:44:03.560000000Z"

func seedSameMillisecondRows(t *testing.T, host, container string) []string {
	t.Helper()
	_ = os.RemoveAll("leveldb/hosts/" + host)
	t.Cleanup(func() { _ = os.RemoveAll("leveldb/hosts/" + host) })

	vars.Mutex.Lock()
	delete(vars.Container_Stat_Counter, host+"/"+container)
	vars.Mutex.Unlock()

	db, err := leveldb.OpenFile("leveldb/hosts/"+host+"/containers/"+container+"/logs", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	messages := []string{"first in the millisecond", "second in the millisecond", "third in the millisecond"}
	for _, message := range messages {
		if err := PutLogMessage(db, host, container, []string{testYear + sameMillisecond, message}); err != nil {
			t.Fatal(err)
		}
	}
	return messages
}

func rowKey(row []string) string {
	if len(row) < 3 {
		return ""
	}
	return row[2]
}

func pageEverything(t *testing.T, getPrev bool, host, container string, pageSize int) []string {
	t.Helper()

	var seen []string
	cursor := ""
	for round := 0; round < 20; round++ {
		result := GetLogs(getPrev, false, host, container, "", pageSize, cursor, false, nil)
		rows := result["logs"].([][]string)
		for _, row := range rows {
			seen = append(seen, row[1])
		}
		if len(rows) == 0 || result["is_end"] == true {
			break
		}

		next := result["last_processed_key"].(string)
		if next == "" || next == cursor {
			t.Fatalf("the cursor did not advance: %q", next)
		}
		cursor = next
	}
	return seen
}

func assertEachExactlyOnce(t *testing.T, label string, seen []string, expected []string) {
	t.Helper()

	counts := map[string]int{}
	for _, message := range seen {
		counts[message]++
	}
	for _, message := range expected {
		switch counts[message] {
		case 0:
			t.Errorf("%s: row %q was never returned", label, message)
		case 1:
		default:
			t.Errorf("%s: row %q was returned %d times", label, message, counts[message])
		}
	}
	if len(seen) != len(expected) {
		t.Errorf("%s: paged %d rows, expected %d: %v", label, len(seen), len(expected), seen)
	}
}

func TestSameMillisecondRowsPageExactlyOnceInBothDirections(t *testing.T) {
	host, container := "CursorHost", "CursorContainer"
	messages := seedSameMillisecondRows(t, host, container)

	// One row per page forces the cursor to land inside the millisecond group.
	backwards := pageEverything(t, false, host, container, 1)
	assertEachExactlyOnce(t, "newest-first", backwards, messages)

	forwards := pageEverything(t, true, host, container, 1)
	assertEachExactlyOnce(t, "oldest-first", forwards, messages)
}

func TestGetLogsReturnsAStableRowIdentity(t *testing.T) {
	host, container := "IdentityHost", "IdentityContainer"
	seedSameMillisecondRows(t, host, container)

	rows := GetLogs(false, false, host, container, "", 30, "", false, nil)["logs"].([][]string)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	keys := map[string]struct{}{}
	for _, row := range rows {
		key := rowKey(row)
		if key == "" {
			t.Fatalf("row %v carries no stable key, so the client cannot deduplicate or page precisely", row)
		}
		if _, exists := keys[key]; exists {
			t.Fatalf("two rows share the key %q", key)
		}
		keys[key] = struct{}{}
	}
}

func TestGetLogsWithAStaleFutureCursorReturnsNothing(t *testing.T) {
	host, container := "StaleCursorHost", "StaleCursorContainer"
	seedSameMillisecondRows(t, host, container)

	result := GetLogs(true, false, host, container, "", 30, "2999-01-01T00:00:00.000000000Z", false, nil)
	rows := result["logs"].([][]string)
	if len(rows) != 0 {
		t.Fatalf("a cursor newer than every row returned %d rows; the iterator teleported back to the newest page: %v", len(rows), rows)
	}
	if result["is_end"] != true {
		t.Errorf("expected is_end=true for an exhausted forward scan, got %v", result["is_end"])
	}
}
