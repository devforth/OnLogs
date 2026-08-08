package containerdb

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
)

// One classification rule, mirrored by classifyLogLine in
// frontend/src/Views/Logs/functions.js.
var logLevels = []struct {
	needle string
	level  string
}{
	{"ERROR", "error"},
	{"ERR", "error"},
	{"WARNING", "warn"},
	{"WARN", "warn"},
	{"DEBUG", "debug"},
	{"INFO", "info"},
	{"ONLOGS", "meta"},
}

func GetLogStatusKey(message string) string {
	for _, token := range strings.Fields(ansiEscapeRegex.ReplaceAllString(message, "")) {
		upper := strings.ToUpper(token)
		for _, level := range logLevels {
			if strings.Contains(upper, level.needle) {
				return level.level
			}
		}
	}
	return "other"
}

func forEachContainer(visit func(host, container string)) error {
	hosts, err := os.ReadDir("leveldb/hosts/")
	if err != nil {
		return fmt.Errorf("failed to read hosts directory: %v", err)
	}
	for _, h := range hosts {
		containers, _ := os.ReadDir("leveldb/hosts/" + h.Name() + "/containers")
		for _, c := range containers {
			visit(h.Name(), c.Name())
		}
	}
	return nil
}

// scanLimit 0 means scan every row.
func pruneUpTo(db *leveldb.DB, cutoff time.Time, scanLimit int) int {
	batch := new(leveldb.Batch)
	deleted := 0

	iter := db.NewIterator(nil, nil)
	scanned := 0
	for ok := iter.First(); ok && (scanLimit == 0 || scanned < scanLimit); ok = iter.Next() {
		scanned++
		keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(iter.Key())))
		if err != nil {
			fmt.Println("Error parsing key time:", err)
			continue
		}
		if !keyTime.After(cutoff) {
			batch.Delete(iter.Key())
			deleted++
		}
	}
	iter.Release()

	if deleted == 0 {
		return 0
	}
	if err := db.Write(batch, nil); err != nil {
		fmt.Println("Failed to delete batch:", err)
		return 0
	}
	db.CompactRange(leveldbUtil.Range{Start: nil, Limit: nil})
	return deleted
}

func checkAndManageLogSize(host string, container string) error {
	maxSize, err := util.ParseHumanReadableSize(os.Getenv("MAX_LOGS_SIZE"))
	if err != nil {
		return fmt.Errorf("failed to parse MAX_LOGS_SIZE: %v", err)
	}

	for {
		var totalSize int64
		if err := forEachContainer(func(h, c string) {
			totalSize += int64(util.GetDirSize(h, c) * 1024 * 1024)
		}); err != nil {
			return err
		}
		if totalSize <= maxSize {
			return nil
		}

		var cutoffKeys [][]byte
		if err := forEachContainer(func(h, c string) {
			logsDB := util.GetDB(h, c, "logs")
			if logsDB == nil {
				return
			}
			if key, err := getCutoffKeysForContainer(logsDB, 200); err == nil && len(key) > 0 {
				cutoffKeys = append(cutoffKeys, key)
			}
		}); err != nil {
			return err
		}

		if len(cutoffKeys) == 0 {
			fmt.Println("Nothing to delete, cutoff keys not found.")
			return nil
		}

		oldestTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(findOldestCutoffKey(cutoffKeys))))
		if err != nil {
			fmt.Println("Error parsing oldest time:", err)
			return nil
		}
		fmt.Println("Oldest time for deletion cutoff:", oldestTime)

		if err := forEachContainer(func(h, c string) {
			// Everything the quota measures, or it can never come down.
			for _, dbType := range util.PrunableDBs() {
				db := util.GetDB(h, c, dbType)
				if db == nil {
					continue
				}
				scanLimit := 0
				if dbType == "logs" {
					scanLimit = 200
				}
				if deleted := pruneUpTo(db, oldestTime, scanLimit); deleted > 0 && dbType == "logs" {
					CountRetentionDeleted(deleted)
					fmt.Printf("Deleted %d logs from %s/%s\n", deleted, h, c)
				}
			}
		}); err != nil {
			return err
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func getCutoffKeysForContainer(db *leveldb.DB, limit int) ([]byte, error) {
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	var cutoffKeys [][]byte
	for ok := iter.First(); ok && len(cutoffKeys) < limit; ok = iter.Next() {
		key := append([]byte{}, iter.Key()...)
		cutoffKeys = append(cutoffKeys, key)
	}

	if len(cutoffKeys) < limit {
		return nil, fmt.Errorf("insufficient records to form cutoff keys")
	}

	return cutoffKeys[len(cutoffKeys)-1], nil
}

func findOldestCutoffKey(cutoffKeys [][]byte) []byte {
	var oldestKey []byte
	var oldestTime time.Time
	first := true

	for _, key := range cutoffKeys {
		keyStr := string(key)
		keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(keyStr))
		if err != nil {
			fmt.Println("Error parsing key time:", err)
			continue
		}

		if first || keyTime.Before(oldestTime) {
			oldestKey = key
			oldestTime = keyTime
			first = false
		}
	}
	return oldestKey
}

const (
	defaultLogsPerRequest = 30
	maxLogsPerRequest     = 1000
)

// Bounds one scan. A var so tests can exercise the cap without a million rows.
var maxScanIterations = 1000000

var (
	logCleanupMu     sync.Mutex
	nextCleanup      time.Time
	isCleanupRunning bool
	ansiEscapeRegex  = regexp.MustCompile(`[\x1B\x9B][[\]()#;?]*(?:(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><])`)
	logKeyCounter    atomic.Uint64
)

func buildLogKey(timestamp string) string {
	return timestamp + " +" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(logKeyCounter.Add(1), 10)
}

func MaybeScheduleCleanup(host string, container string) {
	logCleanupMu.Lock()

	defer logCleanupMu.Unlock()

	if isCleanupRunning {
		return
	}
	if time.Now().Before(nextCleanup) {
		return
	}

	isCleanupRunning = true

	go func() {
		err := checkAndManageLogSize(host, container)

		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()

		isCleanupRunning = false
		nextCleanup = time.Now().Add(1 * time.Minute)

		if err != nil {
			fmt.Printf("Log cleanup failed: %v\n", err)
		}
	}()
}

func NewStatCounter() map[string]uint64 {
	return map[string]uint64{"error": 0, "debug": 0, "info": 0, "warn": 0, "meta": 0, "other": 0}
}

// Bounded because a CI pipeline that mints a container name per release would
// otherwise grow this without limit.
const maxTrackedContainers = 1000

// Process-lifetime counts, unlike vars.Container_Stat_Counter which the
// statistics worker zeroes every minute.
var (
	logLineCounts      = map[string]map[string]uint64{}
	logLineCapReported bool
)

func countLogStatus(location string, statusKey string) {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()

	if vars.Container_Stat_Counter[location] == nil {
		vars.Container_Stat_Counter[location] = NewStatCounter()
	}
	vars.Container_Stat_Counter[location][statusKey]++

	total := logLineCounts[location]
	if total == nil {
		if len(logLineCounts) >= maxTrackedContainers {
			if !logLineCapReported {
				logLineCapReported = true
				fmt.Printf("WARNING: tracking log line counts for %d containers; %s and later ones are not counted in /metrics\n", maxTrackedContainers, location)
			}
			return
		}
		total = NewStatCounter()
		logLineCounts[location] = total
	}
	total[statusKey]++
}

var retentionDeleted atomic.Uint64

func CountRetentionDeleted(lines int) {
	if lines > 0 {
		retentionDeleted.Add(uint64(lines))
	}
}

func RetentionDeletedLines() uint64 {
	return retentionDeleted.Load()
}

// LogLineCounts returns a deep copy: the live maps are written on every log line.
func LogLineCounts() map[string]map[string]uint64 {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()

	snapshot := make(map[string]map[string]uint64, len(logLineCounts))
	for location, levels := range logLineCounts {
		counts := make(map[string]uint64, len(levels))
		for level, value := range levels {
			counts[level] = value
		}
		snapshot[location] = counts
	}
	return snapshot
}

func PutLogMessage(db *leveldb.DB, host string, container string, message_item []string) error {
	if db == nil {
		return fmt.Errorf("no database for %s/%s", host, container)
	}
	if len(message_item) < 2 {
		return fmt.Errorf("malformed log line for %s/%s", host, container)
	}
	if len(message_item[0]) < 30 {
		fmt.Println("WARNING: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return nil
	}

	if host == "" {
		panic("Host is not mentioned!")
	}

	MaybeScheduleCleanup(host, container)

	location := host + "/" + container
	status_key := GetLogStatusKey(message_item[1])
	logKey := buildLogKey(message_item[0])

	// Resolved before taking vars.Mutex: GetDB takes DBMutex and can panic, and
	// it already caches the handle under that lock.
	statusesDB := util.GetDB(host, container, "statuses")

	countLogStatus(location, status_key)

	if statusesDB != nil {
		if err := statusesDB.Put([]byte(logKey), []byte(status_key), nil); err != nil {
			// The handle may have been closed by a concurrent delete; without a
			// retry the line is invisible to every severity filter, forever.
			if reopened := util.GetDB(host, container, "statuses"); reopened != nil {
				reopened.Put([]byte(logKey), []byte(status_key), nil)
			}
		}
	}

	err := db.Put([]byte(logKey), []byte(message_item[1]), nil)
	for tries := 0; err != nil && tries < 10; tries++ {
		time.Sleep(10 * time.Millisecond)
		if reopened := util.GetDB(host, container, "logs"); reopened != nil {
			db = reopened
			err = db.Put([]byte(logKey), []byte(message_item[1]), nil)
		}
	}
	return err
}

func normalizeForSearch(s string, caseSensetivity bool) string {
	s = ansiEscapeRegex.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if !caseSensetivity {
		s = strings.ToLower(s)
	}
	return s
}

func fitsNormalizedSearch(logLine string, normalizedMessage string, caseSensetivity bool) bool {
	if normalizedMessage == "" {
		return true
	}
	return strings.Contains(normalizeForSearch(logLine, caseSensetivity), normalizedMessage)
}

func getMoveDirection(getPrev bool, iter iterator.Iterator) func() bool {
	if !getPrev {
		return func() bool { return iter.Prev() }
	}
	return func() bool { return iter.Next() }
}

// Positions the iterator for the requested direction. A failed Seek means no key
// is >= startWith: walking newer there is genuinely finished, while walking older
// should start from the newest row. goleveldb leaves a failed Seek at dirEOI,
// where Prev() silently jumps to Last() — so the direction must be explicit.
func searchInit(iter iterator.Iterator, startWith string, getPrev bool) bool {
	if startWith == "" {
		if getPrev {
			return iter.First()
		}
		return iter.Last()
	}

	if iter.Seek([]byte(startWith)) {
		return true
	}
	if getPrev {
		return false
	}
	return iter.Last()
}

func getDateTimeFromKey(key string) string {
	return strings.Split(key, " +")[0]
}

/*
Get logs line by line from container.
  - getPrev - if true, will get logs from latest to oldest.
  - include - if true, will include logs with startWith key.

returns json obj like this:

	{
		"logs": [["2021-09-01T12:00:00", "logline"], ["2021-09-01T12:00:01", "logline"]],
		"last_processed_key": "2021-09-01T12:00:01",
		"is_end": false
	}
*/
func GetLogs(getPrev bool, include bool, host string, container string, message string, limit int, startWith string, caseSensetivity bool, status *string) map[string]interface{} {
	if limit <= 0 {
		limit = defaultLogsPerRequest
	} else if limit > maxLogsPerRequest {
		limit = maxLogsPerRequest
	}

	logs_db := util.GetDBIfExists(host, container, "logs")
	if logs_db == nil {
		return map[string]interface{}{"logs": [][]string{}, "last_processed_key": "", "is_end": true}
	}

	var statusDb *leveldb.DB
	if status != nil {
		statusDb = util.GetDBIfExists(host, container, "statuses")
	}
	iter := logs_db.NewIterator(nil, nil)
	defer iter.Release()

	to_return := map[string]interface{}{
		"logs": [][]string{},
	}
	logs := [][]string{}
	move_direction := getMoveDirection(getPrev, iter)

	if !searchInit(iter, startWith, getPrev) {
		to_return["is_end"] = true
		return to_return
	}

	counter := 0
	iteration := 0
	last_processed_key := ""
	last_visited_key := ""
	normalizedMessage := normalizeForSearch(message, caseSensetivity)
	hitScanCap := false
	for counter < limit {
		if iteration >= maxScanIterations {
			hitScanCap = true
			break
		}
		iteration += 1
		key := iter.Key()
		if len(key) == 0 {
			to_return["is_end"] = true
			counter++
			move_direction()
			continue
		} else {
			to_return["is_end"] = false
		}

		keyStr := string(key)
		last_visited_key = keyStr
		timeStr := getDateTimeFromKey(keyStr)
		// Only the cursor row is skipped. Statistics cursors arrive without the
		// " +<nanos>-<counter>" suffix, which no full key can ever equal, so
		// they are matched on that boundary instead.
		if !include && (keyStr == startWith || strings.HasPrefix(keyStr, startWith+" +")) {
			move_direction()
			continue
		}
		value := string(iter.Value())

		if status != nil {
			statusValue, err := statusDb.Get(key, nil)
			if err != nil || string(statusValue) != *status {
				move_direction()
				continue
			}
		}

		if !fitsNormalizedSearch(value, normalizedMessage, caseSensetivity) {
			move_direction()
			continue
		}

		logs = append(logs, []string{timeStr, value, keyStr})
		counter++
		move_direction()
		last_processed_key = keyStr
	}

	if hitScanCap {
		// Cut short rather than exhausted. is_end already reflects whether the
		// iterator ran out, so only the cursor needs filling in: hand back the
		// last key VISITED so the next request resumes past it. An empty cursor
		// would restart the client at the newest row and loop forever.
		to_return["scan_capped"] = true
		if last_processed_key == "" {
			last_processed_key = last_visited_key
		}
	}

	to_return["logs"] = logs
	to_return["last_processed_key"] = last_processed_key
	return to_return
}

func DeleteContainer(host string, container string, fullDelete bool) {
	// Both callers are bare goroutines, so this rejects rather than panics.
	if !util.IsSafeName(host) || !util.IsSafeName(container) {
		fmt.Println("ERROR: refusing to delete container with unsafe name:", host, container)
		return
	}

	for _, dbType := range []string{"logs", "statuses", "statistics", "streamstate"} {
		util.ResetDB(host, container, dbType)
	}

	path := "leveldb/hosts/" + host + "/containers/" + container
	if fullDelete {
		os.RemoveAll(path)
	} else {
		files, _ := os.ReadDir(path)
		for _, file := range files {
			os.RemoveAll(path + "/" + file.Name())
		}
	}

	for _, dbType := range []string{"logs", "statuses", "statistics", "streamstate"} {
		util.ResetDB(host, container, dbType)
	}

	vars.Mutex.Lock()
	vars.Container_Stat_Counter[host+"/"+container] = NewStatCounter()
	// Dropped, not zeroed: frees a slot against maxTrackedContainers.
	delete(logLineCounts, host+"/"+container)
	vars.Mutex.Unlock()
}
