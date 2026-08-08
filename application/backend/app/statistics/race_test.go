package statistics

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/vars"
)

// Mirrors containerdb.countLogStatus: the writer that runs on every log line.
func countConcurrently(location string, stop <-chan struct{}, done chan<- struct{}) {
	for {
		select {
		case <-stop:
			close(done)
			return
		default:
		}
		vars.Mutex.Lock()
		if vars.Container_Stat_Counter[location] == nil {
			vars.Container_Stat_Counter[location] = map[string]uint64{}
		}
		vars.Container_Stat_Counter[location]["error"]++
		vars.Mutex.Unlock()
	}
}

func TestGetStatisticsByServiceDoesNotHandOutTheLiveCounter(t *testing.T) {
	host, service := "StatRaceHost", "StatRaceCont"
	location := host + "/" + service
	_ = os.RemoveAll("leveldb/hosts/" + host)

	seedCounter(location, containerdb.NewStatCounter())

	stop := make(chan struct{})
	done := make(chan struct{})
	go countConcurrently(location, stop, done)

	// value < 1 is the path /getStats takes for the live counters, and the
	// returned map is then marshalled by the handler with no lock held.
	for i := 0; i < 300; i++ {
		result := GetStatisticsByService(host, service, 0)
		for _, key := range []string{"error", "debug", "info", "warn", "meta", "other"} {
			_ = result[key]
		}
	}

	close(stop)
	<-done
}

func TestGetChartDataDoesNotHandOutTheLiveCounter(t *testing.T) {
	host, service := "ChartRaceHost", "ChartRaceCont"
	location := host + "/" + service
	_ = os.RemoveAll("leveldb/hosts/" + host)

	seedCounter(location, containerdb.NewStatCounter())

	stop := make(chan struct{})
	done := make(chan struct{})
	go countConcurrently(location, stop, done)

	for i := 0; i < 200; i++ {
		result := GetChartData(host, service, "hour", 2)
		if now := result["now"]; now != nil {
			for _, key := range []string{"error", "debug", "info", "warn", "meta", "other"} {
				_ = now[key]
			}
		}
	}

	close(stop)
	<-done
}
