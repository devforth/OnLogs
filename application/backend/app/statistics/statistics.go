package statistics

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
)

// Held by a flush across save and reset, and by readers of both sources.
var flushMu sync.RWMutex

// One flush per location: two reading the same cursor count lines twice.
var flushLocks sync.Map

var (
	afterStatsRead   func()
	beforeStatsSave  func(location string)
	beforeStatsReset func(location string)
)

// The caller must never receive the live map: it is written on every log line.
func snapshotCounter(location string) map[string]uint64 {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()

	snapshot := containerdb.NewStatCounter()
	for key, value := range vars.Container_Stat_Counter[location] {
		snapshot[key] = value
	}
	return snapshot
}

func restartStats(host string, container string) {
	current_db := util.GetDB(host, container, "statistics")
	if current_db == nil {
		return
	}
	location := host
	if container != "" {
		location += "/" + container
	}
	lock, _ := flushLocks.LoadOrStore(location, new(sync.Mutex))
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	last_stat_time := getLastStatTime(current_db)
	batch := new(leveldb.Batch)
	if last_stat_time == "" {
		// Anchored to the newest line actually seen, never to the clock: the
		// clock is ahead of lines docker has not replayed yet, and the forward
		// scan would then seek straight past them.
		hours, newest := collectLogs(host, container, "", false)
		flushMu.Lock()
		defer flushMu.Unlock()
		if newest != "" {
			key := saveHours(batch, hours)
			// Late counts are dropped: the full scan counted every stored row.
			containerdb.TakeLateStats(location, key)
			current_db.Write(batch, nil)
		}
	} else {
		hours, new_datetime := collectLogs(host, container, last_stat_time, true)
		flushMu.Lock()
		defer flushMu.Unlock()
		// Nothing new: saving would rewrite the previous interval's record with
		// an empty one and move the cursor off the log timeline onto the clock.
		cursor, key := last_stat_time, ""
		if last_stat_time != new_datetime {
			key = saveHours(batch, hours)
			cursor = max(cursor, key)
		}
		late := containerdb.TakeLateStats(location, cursor)
		if key != "" {
			if beforeStatsSave != nil {
				beforeStatsSave(location)
			}
			current_db.Write(batch, nil)
		}
		if len(late) > 0 {
			addStats(current_db, late, last_stat_time)
		}
	}

	if beforeStatsReset != nil {
		beforeStatsReset(location)
	}
	resetInMemoryStats(location)
}

func readStats(db *leveldb.DB, location string) (map[string]uint64, iterator.Iterator) {
	flushMu.RLock()
	live, iter := snapshotCounter(location), db.NewIterator(nil, nil)
	flushMu.RUnlock()
	if afterStatsRead != nil {
		afterStatsRead()
	}
	return live, iter
}

func getLastStatTime(db *leveldb.DB) string {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	if !iter.Last() {
		return ""
	}

	return string(iter.Key())
}

type hourStats struct {
	newest string
	stats  map[string]uint64
}

// A forward scan returns the cursor it finished on; a backward scan returns the
// newest key it saw, which is where a first cursor belongs.
func collectLogs(host, container, cursor string, forward bool) (map[string]*hourStats, string) {
	hours := map[string]*hourStats{}
	newest := ""

	for {
		page := containerdb.GetLogs(forward, false, host, container, "", 1000, cursor, true, nil)
		logs, ok := page["logs"].([][]string)
		if !ok || len(logs) == 0 {
			break
		}

		if !forward && newest == "" {
			newest = logs[0][2]
		}

		for _, log := range logs {
			hour := ""
			if _, err := time.Parse(time.RFC3339Nano, log[0]); err == nil {
				hour = strings.Split(log[0], ":")[0]
			}
			h := hours[hour]
			if h == nil {
				h = &hourStats{stats: containerdb.NewStatCounter()}
				hours[hour] = h
			}
			h.stats[containerdb.GetLogStatusKey(log[1])]++
			if log[2] > h.newest {
				h.newest = log[2]
			}
		}

		cursor = page["last_processed_key"].(string)
		if page["is_end"].(bool) {
			break
		}
	}

	if odd := hours[""]; odd != nil {
		top := odd
		for _, h := range hours {
			if h.newest > top.newest {
				top = h
			}
		}
		if top != odd {
			for level, count := range odd.stats {
				top.stats[level] += count
			}
			delete(hours, "")
		}
	}

	if forward {
		return hours, cursor
	}
	return hours, newest
}

func saveHours(batch *leveldb.Batch, hours map[string]*hourStats) string {
	newest := ""
	for _, h := range hours {
		newest = max(newest, saveStats(batch, h.stats, h.newest))
	}
	return newest
}

// Statistics are keyed by time and read back with time.Parse, so a full log key
// (which carries a " +<nano>-<counter>" suffix) must be reduced to its timestamp.
func statKey(timestamp string) string {
	key := strings.Split(timestamp, " +")[0]
	if _, err := time.Parse(time.RFC3339Nano, key); err != nil {
		key = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return key
}

func saveStats(batch *leveldb.Batch, stats map[string]uint64, timestamp string) string {
	key := statKey(timestamp)
	to_put, _ := json.Marshal(stats)
	batch.Put([]byte(key), to_put)
	return key
}

func addStats(db *leveldb.DB, stats map[string]uint64, timestamp string) {
	key := statKey(timestamp)
	if old, err := db.Get([]byte(key), nil); err == nil {
		record := map[string]uint64{}
		if json.Unmarshal(old, &record) == nil {
			for level, count := range record {
				stats[level] += count
			}
		}
	}

	to_put, _ := json.Marshal(stats)
	db.Put([]byte(key), to_put, nil)
}

func resetInMemoryStats(location string) {
	vars.Mutex.Lock()
	vars.Container_Stat_Counter[location] = containerdb.NewStatCounter()
	vars.Mutex.Unlock()
}

func RunStatisticForContainerWithContext(ctx context.Context, host string, container string) {
	location := host + "/" + container
	resetInMemoryStats(location)
	defer restartStats(host, container)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		restartStats(host, container)
		select {
		case <-ctx.Done():
			return
		case <-time.After(vars.StatisticsSaveInterval):
		}
	}
}

func GetStatisticsByService(host string, service string, value int) map[string]uint64 {
	location := host + "/" + service
	if value < 1 {
		return snapshotCounter(location)
	}

	searchTo := time.Now().Add(-(time.Hour * time.Duration(value/2))).UTC()
	current_db := util.GetDBIfExists(host, service, "statistics")
	if current_db == nil {
		return snapshotCounter(location)
	}
	to_return, iter := readStats(current_db, location)
	defer iter.Release()
	iter.Last()

	for hasPrev := true; hasPrev; hasPrev = iter.Prev() {
		tmp_time, _ := time.Parse(time.RFC3339Nano, string(iter.Key()))
		if searchTo.After(tmp_time) {
			break
		}
		// Fresh each round: Unmarshal merges into a non-nil map.
		record := map[string]uint64{}
		json.Unmarshal(iter.Value(), &record)
		for level, count := range record {
			to_return[level] += count
		}
	}
	return to_return
}

func GetChartData(host string, service string, unit string, uAmount int) map[string]map[string]uint64 {
	var searchTo time.Time
	var sep, formatting string
	if unit == "hour" {
		searchTo = time.Now().Add(-(time.Hour * time.Duration(uAmount)))
		sep = ":"
		formatting = ":00Z"
	} else if unit == "day" {
		searchTo = time.Now().AddDate(0, 0, -uAmount)
		sep = "T"
		formatting = "T00:00Z"
	} else if unit == "month" {
		searchTo = time.Now().AddDate(0, -uAmount, 0)
		formatting = "-01T00:00Z"
	} else {
		return nil
	}

	location := host + "/" + service
	to_return := map[string]map[string]uint64{}
	statsDB := util.GetDBIfExists(host, service, "statistics")
	if statsDB == nil {
		return to_return
	}
	live, iter := readStats(statsDB, location)
	iter.Last()
	defer iter.Release()
	hasPrev := true
	for hasPrev {
		tmp_time, _ := time.Parse(time.RFC3339Nano, string(iter.Key()))
		if searchTo.After(tmp_time) {
			break
		}

		var datetime string
		if unit == "month" {
			datetime = string(iter.Key())[:7] + formatting
		} else {
			datetime = strings.Split(string(iter.Key()), sep)[0] + formatting
		}
		if to_return[datetime] == nil {
			to_return[datetime] = containerdb.NewStatCounter()
		}
		record := map[string]uint64{}
		json.Unmarshal(iter.Value(), &record)
		for level, count := range record {
			to_return[datetime][level] += count
		}

		hasPrev = iter.Prev()
	}

	to_return["now"] = live

	return to_return
}
