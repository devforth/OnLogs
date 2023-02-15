package statistics

import (
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/vars"
)

func TestRunStatisticForHost(t *testing.T) {
	go RunStatisticForHost("Test")
	time.Sleep(1 * time.Second)
	if vars.Counters_For_Hosts_Last_30_Min["Test"] == nil {
		t.Error("No counter variable for host was created!")
	}
	vars.Counters_For_Hosts_Last_30_Min["Test"] = nil
}

func TestRunStatisticForContainer(t *testing.T) {
	go RunStatisticForHost("Test")
	go RunStatisticForContainer("Test", "TestContainer")
	time.Sleep(1 * time.Second)
	if vars.Counters_For_Containers_Last_30_Min["Test/TestContainer"] == nil {
		t.Error("No counter variable for container was created!")
	}
	if vars.Stat_Containers_DBs["Test/TestContainer"] == nil {
		t.Error("DB for stats wasn't created!")
	}
}
