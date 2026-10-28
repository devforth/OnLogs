package containerdb

import (
	"bytes"
	"errors"
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
	"github.com/syndtr/goleveldb/leveldb/opt"
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

const maxRoundBytes = 2 << 20

const minRoundRows = 200

var errNoRoom = errors.New("not enough free disk space to compact")

func pruneUpTo(db *leveldb.DB, cutoff time.Time, scanLimit int, also ...*leveldb.DB) ([][]byte, error) {
	var keys [][]byte
	iter := db.NewIterator(nil, nil)
	scanned, size := 0, 0
	for ok := iter.First(); ok && (scanLimit == 0 || scanned < scanLimit) && size < maxRoundBytes; ok = iter.Next() {
		scanned++
		keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(iter.Key())))
		if err != nil {
			fmt.Println("Error parsing key time:", err)
			continue
		}
		if keyTime.After(cutoff) {
			break
		}
		keys = append(keys, append([]byte{}, iter.Key()...))
		size += len(iter.Key()) + 8
	}
	iter.Release()

	if len(keys) == 0 {
		return nil, nil
	}
	if err := checkRoom(size, append(also, db)...); err != nil {
		return nil, err
	}
	written, err := applyDeletes(db, keys)
	return keys[:written], err
}

func checkRoom(tombstones int, dbs ...*leveldb.DB) error {
	need := uint64(opt.DefaultWriteBuffer)
	for _, db := range dbs {
		if db == nil {
			continue
		}
		var stats leveldb.DBStats
		if err := db.Stats(&stats); err != nil {
			return err
		}
		need += uint64(tombstones)
		for _, size := range stats.LevelSizes[:min(2, len(stats.LevelSizes))] {
			need += uint64(size)
		}
	}
	if free, err := util.FreeDiskBytes(); err == nil && free < need {
		return fmt.Errorf("%w: %d bytes free, %d needed", errNoRoom, free, need)
	}
	return nil
}

func applyDeletes(db *leveldb.DB, keys [][]byte) (written int, err error) {
	batch := new(leveldb.Batch)
	size := 0
	for i, key := range keys {
		batch.Delete(key)
		size += len(key) + 8
		// A Write over 4 MiB becomes a transaction, which keeps its lock if it fails.
		if size >= 1<<20 || i == len(keys)-1 {
			if err := db.Write(batch, nil); err != nil {
				return written, err
			}
			written, size = i+1, 0
			batch.Reset()
		}
	}
	// Not the whole DB: with a fresh line in the memtable that rewrites every table.
	return written, db.CompactRange(leveldbUtil.Range{Limit: keys[len(keys)-1]})
}

type retentionRound struct {
	scan   map[string]int
	cutoff time.Time
	raw    int64
	whole  bool
}

func oldestRows(want int64) (retentionRound, error) {
	type head struct {
		location string
		iter     iterator.Iterator
		ok       bool
		scanned  int
	}
	var heads []*head
	defer func() {
		for _, h := range heads {
			h.iter.Release()
		}
	}()
	round := retentionRound{scan: map[string]int{}}
	if err := forEachContainer(func(h, c string) {
		if db := util.GetDB(h, c, "logs"); db != nil {
			iter := db.NewIterator(nil, nil)
			heads = append(heads, &head{location: h + "/" + c, iter: iter, ok: iter.First()})
		}
	}); err != nil {
		return round, err
	}

	rows, tombstones := 0, 0
	for tombstones < maxRoundBytes && (rows < minRoundRows || round.raw < want) {
		var next *head
		for _, h := range heads {
			if h.ok && (next == nil || bytes.Compare(h.iter.Key(), next.iter.Key()) < 0) {
				next = h
			}
		}
		if next == nil {
			round.whole = true
			break
		}
		key := next.iter.Key()
		next.scanned++
		if keyTime, err := time.Parse(time.RFC3339Nano, getDateTimeFromKey(string(key))); err == nil {
			rows++
			tombstones += len(key) + 8
			round.raw += int64(len(key) + len(next.iter.Value()))
			round.scan[next.location] = next.scanned
			if keyTime.After(round.cutoff) {
				round.cutoff = keyTime
			}
		}
		next.ok = next.iter.Next()
	}
	return round, nil
}

func pruneContainer(h, c string, cutoff time.Time, scan int) (failed error) {
	var deleted [][]byte
	// Everything the quota measures, or it can never come down.
	for _, dbType := range util.PrunableDBs() {
		beforePrune(h, c, dbType)
		db := util.GetDB(h, c, dbType)
		if db == nil {
			continue
		}
		var err error
		switch dbType {
		case "logs":
			if scan == 0 {
				continue
			}
			if deleted, err = pruneUpTo(db, cutoff, scan, util.GetDB(h, c, "statuses")); len(deleted) > 0 {
				CountRetentionDeleted(len(deleted))
				fmt.Printf("Deleted %d logs from %s/%s\n", len(deleted), h, c)
			}
		case "statuses":
			if len(deleted) > 0 {
				_, err = applyDeletes(db, deleted)
			}
			if err == nil && failed == nil {
				err = pruneStatusesWithoutRows(db, util.GetDB(h, c, "logs"))
			}
		default:
			if failed == nil {
				_, err = pruneUpTo(db, cutoff, 0)
			}
		}
		if err != nil {
			if !errors.Is(err, errNoRoom) {
				util.ReopenDB(h, c, dbType, db)
			}
			if failed == nil {
				failed = fmt.Errorf("pruning %s of %s/%s: %w", dbType, h, c, err)
			}
		}
	}
	return failed
}

func pruneStatusesWithoutRows(statuses, logs *leveldb.DB) error {
	if logs == nil {
		return nil
	}
	var first []byte
	iter := logs.NewIterator(nil, nil)
	if iter.First() {
		first = append([]byte{}, iter.Key()...)
	}
	unreadable := iter.Error() != nil
	iter.Release()
	if unreadable {
		return nil
	}

	var keys [][]byte
	size := 0
	iter = statuses.NewIterator(&leveldbUtil.Range{Limit: first}, nil)
	for size < maxRoundBytes && iter.Next() {
		keys = append(keys, append([]byte{}, iter.Key()...))
		size += len(iter.Key()) + 8
	}
	iter.Release()
	if len(keys) == 0 {
		return nil
	}
	if err := checkRoom(size, statuses); err != nil {
		return err
	}
	_, err := applyDeletes(statuses, keys)
	return err
}

func removeEmptyHosts() {
	hosts, _ := os.ReadDir("leveldb/hosts/")
	for _, h := range hosts {
		if !h.IsDir() || h.Name() == util.GetHost() || len(vars.AgentContainers(h.Name())) > 0 {
			continue
		}
		if removed, err := util.RemoveHostIfEmpty(h.Name()); err != nil {
			fmt.Printf("Failed to remove empty host %s: %v\n", h.Name(), err)
		} else if removed {
			fmt.Println("Removed empty host", h.Name())
		}
	}
}

func checkAndManageLogSize(host string, container string) error {
	maxSize, err := util.ParseHumanReadableSize(os.Getenv("MAX_LOGS_SIZE"))
	if err != nil {
		return fmt.Errorf("failed to parse MAX_LOGS_SIZE: %v", err)
	}

	ratio := 1.0
	var lastSize, lastRaw int64
	for {
		var totalSize int64
		if err := forEachContainer(func(h, c string) {
			totalSize += int64(util.GetPrunableSize(h, c) * 1024 * 1024)
		}); err != nil {
			return err
		}
		if totalSize <= maxSize {
			return nil
		}
		if lastRaw > 0 && lastSize > totalSize {
			ratio = max(float64(lastSize-totalSize)/float64(lastRaw), ratio/2)
		}

		round, err := oldestRows(int64(float64(totalSize-maxSize) / ratio))
		if err != nil {
			return err
		}
		if len(round.scan) == 0 {
			fmt.Println("Nothing to delete, cutoff keys not found.")
			return nil
		}
		fmt.Println("Oldest time for deletion cutoff:", round.cutoff)

		var failed error
		if err := forEachContainer(func(h, c string) {
			if failed == nil {
				failed = pruneContainer(h, c, round.cutoff, round.scan[h+"/"+c])
			}
		}); err != nil {
			return err
		}
		// Ends the run so a full disk is retried after the cooldown, not spun on.
		if failed != nil {
			return failed
		}
		removeEmptyHosts()

		if round.whole {
			return nil
		}
		lastSize, lastRaw = totalSize, round.raw
		time.Sleep(100 * time.Millisecond)
	}
}

const (
	defaultLogsPerRequest = 30
	maxLogsPerRequest     = 1000
)

// Bounds one scan. A var so tests can exercise the cap without a million rows.
var maxScanIterations = 1000000

var cleanupCooldown = time.Minute

var beforePrune = func(host, container, dbType string) {}

var (
	logCleanupMu     sync.Mutex
	nextCleanup      time.Time
	isCleanupRunning bool
	cleanupPending   bool
	ansiEscapeRegex  = regexp.MustCompile(`[\x1B\x9B][[\]()#;?]*(?:(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><])`)
	logKeyCounter    atomic.Uint64
)

func NewEventID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatUint(logKeyCounter.Add(1), 10)
}

func buildLogKey(timestamp string) string {
	return timestamp + " +" + NewEventID()
}

func MaybeScheduleCleanup(host string, container string) {
	logCleanupMu.Lock()

	defer logCleanupMu.Unlock()

	if isCleanupRunning || time.Now().Before(nextCleanup) {
		cleanupPending = true
		return
	}

	isCleanupRunning = true
	cleanupPending = false

	go func() {
		err := checkAndManageLogSize(host, container)

		logCleanupMu.Lock()
		defer logCleanupMu.Unlock()

		isCleanupRunning = false
		nextCleanup = time.Now().Add(cleanupCooldown)
		cleanupPending = cleanupPending || err != nil
		time.AfterFunc(cleanupCooldown, func() {
			logCleanupMu.Lock()
			pending := cleanupPending
			logCleanupMu.Unlock()
			if pending {
				MaybeScheduleCleanup(host, container)
			}
		})

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

// Rows stored at or behind the statistics cursor, which its next scan skips.
var (
	statCursors = map[string]string{}
	lateCounts  = map[string]map[string]*LateHour{}
)

type LateHour struct {
	Newest string
	Stats  map[string]uint64
}

func countIfBehindStatCursor(location string, statusKey string, logKey string) {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()

	cursor := statCursors[location]
	// The forward scan's own boundary: Seek(cursor), then skip rows at cursor.
	if cursor == "" || (logKey >= cursor && !strings.HasPrefix(logKey, cursor+" +")) {
		return
	}
	hour := ""
	if ts := getDateTimeFromKey(logKey); ts != "" {
		if _, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			hour = strings.Split(ts, ":")[0]
		}
	}
	if lateCounts[location] == nil {
		lateCounts[location] = map[string]*LateHour{}
	}
	late := lateCounts[location][hour]
	if late == nil {
		late = &LateHour{Stats: NewStatCounter()}
		lateCounts[location][hour] = late
	}
	late.Stats[statusKey]++
	late.Newest = max(late.Newest, logKey)
}

// Shared by a write from its late check to its store, exclusive while a flush scans.
var ingestLocks sync.Map

func ingestLock(location string) *sync.RWMutex {
	lock, _ := ingestLocks.LoadOrStore(location, new(sync.RWMutex))
	return lock.(*sync.RWMutex)
}

func PauseIngest(location string) (resume func()) {
	lock := ingestLock(location)
	lock.Lock()
	return lock.Unlock
}

func TakeLateStats(location string, cursor string) map[string]*LateHour {
	vars.Mutex.Lock()
	defer vars.Mutex.Unlock()

	statCursors[location] = cursor
	late := lateCounts[location]
	delete(lateCounts, location)
	return late
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
	_, _, err := PutLogEvent(db, host, container, message_item, "")
	return err
}

var eventMu sync.Mutex

func PutLogEvent(db *leveldb.DB, host string, container string, message_item []string, eventID string) (logKey string, duplicate bool, err error) {
	if db == nil {
		return "", false, fmt.Errorf("no database for %s/%s", host, container)
	}
	if len(message_item) < 2 {
		return "", false, fmt.Errorf("malformed log line for %s/%s", host, container)
	}
	if len(message_item[0]) < 30 {
		fmt.Println("WARNING: got broken timestamp: ", "timestamp: "+message_item[0], "message: "+message_item[1])
		return "", false, nil
	}

	if host == "" {
		panic("Host is not mentioned!")
	}

	MaybeScheduleCleanup(host, container)

	location := host + "/" + container
	// Before eventMu, so a paused location never holds up another's writes.
	ingest := ingestLock(location)
	ingest.RLock()
	defer ingest.RUnlock()
	status_key := GetLogStatusKey(message_item[1])
	if eventID == "" {
		logKey = buildLogKey(message_item[0])
	} else {
		logKey = message_item[0] + " +" + eventID
		eventMu.Lock()
		defer eventMu.Unlock()
		if stored, _ := db.Has([]byte(logKey), nil); stored {
			return logKey, true, nil
		}
	}

	// Resolved before taking vars.Mutex: GetDB takes DBMutex and can panic, and
	// it already caches the handle under that lock.
	statusesDB := util.GetDB(host, container, "statuses")

	countLogStatus(location, status_key)
	// Before the write: a row classed against a new cursor missed the scan that set it.
	countIfBehindStatCursor(location, status_key, logKey)

	if statusesDB != nil {
		if err := statusesDB.Put([]byte(logKey), []byte(status_key), nil); err != nil {
			if reopened := util.ReopenDB(host, container, "statuses", statusesDB); reopened != nil {
				reopened.Put([]byte(logKey), []byte(status_key), nil)
			}
		}
	}

	err = db.Put([]byte(logKey), []byte(message_item[1]), nil)
	for tries := 0; err != nil && tries < 10; tries++ {
		if reopened := util.ReopenDB(host, container, "logs", db); reopened != nil && reopened != db {
			db = reopened
		} else {
			time.Sleep(10 * time.Millisecond)
		}
		err = db.Put([]byte(logKey), []byte(message_item[1]), nil)
	}
	return logKey, false, err
}

func LastKeyAt(host string, container string, startWith string) string {
	if startWith == "" || strings.Contains(startWith, " +") {
		return startWith
	}
	logsDB := util.GetDBIfExists(host, container, "logs")
	if logsDB == nil {
		return startWith
	}
	iter := logsDB.NewIterator(leveldbUtil.BytesPrefix([]byte(startWith+" +")), nil)
	defer iter.Release()
	if iter.Last() {
		return string(iter.Key())
	}
	return startWith
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

func rowStatus(statusDb *leveldb.DB, key []byte, value string) string {
	if statusDb != nil {
		if stored, err := statusDb.Get(key, nil); err == nil {
			return string(stored)
		}
	}
	return GetLogStatusKey(value)
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

		if status != nil && rowStatus(statusDb, key, value) != *status {
			move_direction()
			continue
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

const StreamCursorKey = "last_cursor_ts"

func DeleteContainer(host string, container string, fullDelete bool) {
	// Service deletion runs this in a bare goroutine, so it rejects rather than panics.
	if !util.IsSafeName(host) || !util.IsSafeName(container) {
		fmt.Println("ERROR: refusing to delete container with unsafe name:", host, container)
		return
	}

	for _, dbType := range []string{"logs", "statuses", "statistics", "streamstate"} {
		util.ResetDB(host, container, dbType)
	}

	path := "leveldb/hosts/" + host + "/containers/" + container
	cleared := false
	if fullDelete {
		os.RemoveAll(path)
	} else {
		files, err := os.ReadDir(path)
		for _, file := range files {
			os.RemoveAll(path + "/" + file.Name())
		}
		cleared = err == nil
	}

	for _, dbType := range []string{"logs", "statuses", "statistics", "streamstate"} {
		util.ResetDB(host, container, dbType)
	}

	if cleared && host == util.GetHost() {
		if db := util.GetDB(host, container, "streamstate"); db != nil {
			if err := db.Put([]byte(StreamCursorKey), []byte(time.Now().UTC().Format(time.RFC3339Nano)), nil); err != nil {
				fmt.Println("ERROR: unable to save stream cursor:", err)
			}
		}
	}

	vars.Mutex.Lock()
	vars.Container_Stat_Counter[host+"/"+container] = NewStatCounter()
	// Dropped, not zeroed: frees a slot against maxTrackedContainers.
	delete(logLineCounts, host+"/"+container)
	vars.Mutex.Unlock()
}
