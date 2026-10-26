package statistics_test

import (
	"maps"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
)

func TestStatsCountABacklogInTheHoursItWasLogged(t *testing.T) {
	for name, firstFlush := range map[string]bool{"AfterASavedRecord": false, "OnTheFirstFlush": true} {
		t.Run(name, func(t *testing.T) {
			host, svc := newStatsContainer(t, "Backlog"+name)
			logs := util.GetDB(host, svc, "logs")
			now := time.Now().UTC()
			put := func(ago time.Duration) {
				at := now.Add(-ago).Format("2006-01-02T15:04:05.000000000Z")
				if err := containerdb.PutLogMessage(logs, host, svc, []string{at, "ERROR delivered after an outage"}); err != nil {
					t.Fatal(err)
				}
			}

			lines := uint64(0)
			if !firstFlush {
				put(30 * time.Hour)
				statistics.RestartStats(host, svc)
				lines++
			}
			want := map[string]uint64{}
			for _, ago := range []time.Duration{20 * time.Hour, 15 * time.Hour, 5 * time.Hour, 3 * time.Hour, 10 * time.Minute} {
				put(ago)
				lines++
				if ago <= 12*time.Hour {
					want[now.Add(-ago).Format("2006-01-02T15")+":00Z"]++
				}
			}
			statistics.RestartStats(host, svc)

			got := map[string]uint64{}
			for bucket, levels := range statistics.GetChartData(host, svc, "hour", 12) {
				if bucket != "now" && levels["error"] > 0 {
					got[bucket] = levels["error"]
				}
			}
			if !maps.Equal(got, want) {
				t.Errorf("12-hour chart has errors %v, want %v", got, want)
			}
			if got := statistics.GetStatisticsByService(host, svc, 2)["error"]; got != 1 {
				t.Errorf("last hour has %d errors, want 1", got)
			}
			total := uint64(0)
			for _, levels := range statistics.GetChartData(host, svc, "month", 12) {
				total += levels["error"]
			}
			if total != lines {
				t.Errorf("12-month chart has %d errors, want %d", total, lines)
			}
		})
	}
}

func TestAnUnparsableTimestampKeepsTheStatsCursorOnTheLogTimeline(t *testing.T) {
	host, svc := newStatsContainer(t, "UnparsableTimestamp")
	logs := util.GetDB(host, svc, "logs")
	now := time.Now().UTC()
	put := func(at string) {
		if err := containerdb.PutLogMessage(logs, host, svc, []string{at, "ERROR"}); err != nil {
			t.Fatal(err)
		}
	}
	ago := func(d time.Duration) string { return now.Add(-d).Format("2006-01-02T15:04:05.000000000Z") }

	put(ago(30 * time.Hour))
	statistics.RestartStats(host, svc)
	put(now.Add(-4*time.Hour).Format("2006-01-02 15:04:05.000000000") + " UTC")
	put(ago(5 * time.Hour))
	put(ago(3 * time.Hour))
	statistics.RestartStats(host, svc)
	put(ago(2 * time.Hour))
	statistics.RestartStats(host, svc)

	hour := now.Add(-2*time.Hour).Format("2006-01-02T15") + ":00Z"
	if got := statistics.GetChartData(host, svc, "hour", 12)[hour]["error"]; got != 1 {
		t.Errorf("hour %s has %d errors, want 1", hour, got)
	}
	total := uint64(0)
	for _, levels := range statistics.GetChartData(host, svc, "month", 12) {
		total += levels["error"]
	}
	if total != 5 {
		t.Errorf("12-month chart has %d errors, want 5", total)
	}
}

func TestStatsCountARowStoredBehindTheNewestHourOfABacklog(t *testing.T) {
	host, svc := newStatsContainer(t, "BehindBacklog")
	logs := util.GetDB(host, svc, "logs")
	now := time.Now().UTC()
	put := func(ago time.Duration) {
		at := now.Add(-ago).Format("2006-01-02T15:04:05.000000000Z")
		if err := containerdb.PutLogMessage(logs, host, svc, []string{at, "ERROR"}); err != nil {
			t.Fatal(err)
		}
	}

	put(30 * time.Hour)
	statistics.RestartStats(host, svc)
	put(5 * time.Hour)
	put(10 * time.Minute)
	statistics.RestartStats(host, svc)
	put(3 * time.Hour)
	statistics.RestartStats(host, svc)

	total := uint64(0)
	for _, levels := range statistics.GetChartData(host, svc, "month", 12) {
		total += levels["error"]
	}
	if total != 4 {
		t.Errorf("12-month chart has %d errors, want 4", total)
	}
}
