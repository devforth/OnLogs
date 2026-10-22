package containerdb

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/devforth/OnLogs/app/util"
)

func TestDeleteContainerCannotRemoveATreeOutsideLeveldb(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/realhost/containers", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("victim_tree/nested", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("victim_tree/nested/important", []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	DeleteContainer("realhost", "../../../../victim_tree", true)

	if _, err := os.Stat("victim_tree/nested/important"); err != nil {
		t.Fatalf("an admin delete removed a tree outside leveldb/hosts: %v", err)
	}
}

func TestDeleteContainerLogsCannotEmptyATreeOutsideLeveldb(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll("leveldb/hosts/realhost/containers", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("victim_tree/nested", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("victim_tree/nested/important", []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	DeleteContainer("realhost", "../../../../victim_tree", false)

	if _, err := os.Stat("victim_tree/nested/important"); err != nil {
		t.Fatalf("a clear-logs request emptied a tree outside leveldb/hosts: %v", err)
	}
}

func TestClearingLogsOfAnUnknownServiceDoesNotCreateIt(t *testing.T) {
	t.Chdir(t.TempDir())

	DeleteContainer(util.GetHost(), "never-streamed", false)

	if _, err := os.Stat("leveldb/hosts/" + util.GetHost() + "/containers/never-streamed"); !os.IsNotExist(err) {
		t.Fatalf("clearing an unknown service created it: %v", err)
	}
}

func TestSeverityFilterKeepsLinesWrittenDuringClearLogs(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MAX_LOGS_SIZE", "1TB")
	const host, container = "RaceHost", "RaceContainer"
	t.Cleanup(func() { DeleteContainer(host, container, true) })
	errorLevel := "error"

	for _, fullDelete := range []bool{false, true} {
		for i := 0; i < 50; i++ {
			db := util.GetDB(host, container, "logs")
			line := []string{testYear + "-02-10T12:56:09.230421754Z", fmt.Sprintf("ERROR racing %t %d", fullDelete, i)}
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_ = PutLogMessage(db, host, container, line)
			}()
			go func() {
				defer wg.Done()
				<-start
				DeleteContainer(host, container, fullDelete)
			}()
			close(start)
			wg.Wait()

			all := GetLogs(false, false, host, container, "", 30, "", true, nil)["logs"].([][]string)
			filtered := GetLogs(false, false, host, container, "", 30, "", true, &errorLevel)["logs"].([][]string)
			if len(filtered) != len(all) {
				t.Fatalf("fullDelete=%t: %d rows readable but %d under the error filter: %v", fullDelete, len(all), len(filtered), all)
			}
		}
	}
}

func TestLogsDatabaseOpensWhileClearLogsRemovesIt(t *testing.T) {
	t.Chdir(t.TempDir())
	const host, container = "OpenRaceHost", "OpenRaceContainer"
	t.Cleanup(func() { DeleteContainer(host, container, true) })

	for _, fullDelete := range []bool{false, true} {
		for i := 0; i < 50; i++ {
			seed := util.GetDB(host, container, "logs")
			if seed == nil {
				t.Fatal("could not open the logs database")
			}
			if err := seed.Put([]byte("k"), []byte("v"), nil); err != nil {
				t.Fatal(err)
			}
			util.ResetDB(host, container, "logs")

			opened := false
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				opened = util.GetDB(host, container, "logs") != nil
			}()
			go func() {
				defer wg.Done()
				<-start
				DeleteContainer(host, container, fullDelete)
			}()
			close(start)
			wg.Wait()
			if !opened {
				t.Fatalf("fullDelete=%t: a stream starting at attempt %d got no logs database", fullDelete, i)
			}
		}
	}
}

func TestSeverityFilterFallsBackToTheBodyWhenTheStatusRecordIsGone(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MAX_LOGS_SIZE", "1TB")
	const host, container = "LostStatusHost", "LostStatusContainer"
	t.Cleanup(func() { DeleteContainer(host, container, true) })
	ts := testYear + "-02-10T12:56:09.230421754Z"

	db := util.GetDB(host, container, "logs")
	for _, body := range []string{"ERROR record lost", "INFO record kept"} {
		if err := PutLogMessage(db, host, container, []string{ts, body}); err != nil {
			t.Fatal(err)
		}
	}
	keys := map[string][]byte{}
	for _, row := range GetLogs(false, false, host, container, "", 30, "", true, nil)["logs"].([][]string) {
		keys[row[1]] = []byte(row[2])
	}
	statuses := util.GetDB(host, container, "statuses")
	if err := statuses.Delete(keys["ERROR record lost"], nil); err != nil {
		t.Fatal(err)
	}
	if err := statuses.Put(keys["INFO record kept"], []byte("warn"), nil); err != nil {
		t.Fatal(err)
	}

	for level, want := range map[string]string{"error": "ERROR record lost", "warn": "INFO record kept", "info": ""} {
		rows := GetLogs(false, false, host, container, "", 30, "", true, &level)["logs"].([][]string)
		got := ""
		if len(rows) == 1 {
			got = rows[0][1]
		}
		if len(rows) > 1 || got != want {
			t.Errorf("%s filter returned %v, want %q", level, rows, want)
		}
	}
}
