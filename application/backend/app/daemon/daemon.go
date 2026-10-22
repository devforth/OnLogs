package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devforth/OnLogs/app/agent"
	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/docker"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gorilla/websocket"
	"github.com/syndtr/goleveldb/leveldb"
	leveldbUtil "github.com/syndtr/goleveldb/leveldb/util"
)

const (
	cursorKey           = containerdb.StreamCursorKey
	sentAtCursorKey     = "last_cursor_sent"
	streamTimestampFmt  = "2006-01-02T15:04:05.000000000Z"
	overlapBackfill     = 2 * time.Second
	initialBackfill     = 30 * time.Second
	maxRecentLogEntries = 500

	maxLogLine = 1024 * 1024
	// Keeps an agent's JSON request under the master's 1 MiB body limit.
	keptLongLine      = 128 * 1024
	truncatedLineMark = " [ONLOGS: truncated, the line was over 1 MiB]"
)

var drainTimeout = time.Minute

var errDrainTimeout = errors.New("drain timeout")

type DaemonService struct {
	DockerClient *docker.DockerService

	streamsMu          sync.Mutex
	streamCancels      map[string]context.CancelCauseFunc
	streamIDs          map[string]uint64
	streamSeq          uint64
	recentFingerprints map[string][]string
	recentSet          map[string]map[string]struct{}
	droppedReplays     map[string]int

	// Stopped streams still draining, with the context of a start that came meanwhile.
	draining map[string]context.Context

	// droppedReplays is cleared when a stream ends; these two are for /metrics,
	// which needs a total that survives a restart and a cursor readable without
	// touching LevelDB.
	droppedReplayTotals map[string]uint64
	cursorTimestamps    map[string]time.Time
}

func NewDaemonService(dockerClient *docker.DockerService) *DaemonService {
	return &DaemonService{
		DockerClient:        dockerClient,
		streamCancels:       map[string]context.CancelCauseFunc{},
		streamIDs:           map[string]uint64{},
		draining:            map[string]context.Context{},
		recentFingerprints:  map[string][]string{},
		recentSet:           map[string]map[string]struct{}{},
		droppedReplays:      map[string]int{},
		droppedReplayTotals: map[string]uint64{},
		cursorTimestamps:    map[string]time.Time{},
	}
}

func noteMeta(db *leveldb.DB, host, containerName, token string, toHost bool, message string) {
	if toHost {
		agent.SendLogMessage(token, containerName,
			strings.SplitN(createLogMessage(nil, host, containerName, message), " ", 2))
		return
	}
	createLogMessage(db, host, containerName, message)
}

func createLogMessage(db *leveldb.DB, host string, container string, message string) string {
	datetime := time.Now().UTC().Format(streamTimestampFmt)
	if db != nil {
		containerdb.PutLogMessage(db, host, container, []string{datetime, message})
	}
	return datetime + " " + message
}

func normalizeTimestamp(raw string) (string, time.Time, error) {
	ts, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return "", time.Time{}, err
	}
	ts = ts.UTC()
	return ts.Format(streamTimestampFmt), ts, nil
}

func parseDockerLogLine(line string) ([]string, time.Time, bool) {
	parts := strings.SplitN(strings.TrimRight(line, "\r"), " ", 2)
	if len(parts) != 2 {
		return nil, time.Time{}, false
	}

	tsStr, ts, err := normalizeTimestamp(parts[0])
	if err != nil {
		return nil, time.Time{}, false
	}

	return []string{tsStr, parts[1]}, ts, true
}

func (h *DaemonService) rememberFingerprint(containerName, fingerprint string) {
	if h.recentSet[containerName] == nil {
		h.recentSet[containerName] = map[string]struct{}{}
	}
	if _, exists := h.recentSet[containerName][fingerprint]; exists {
		return
	}

	h.recentSet[containerName][fingerprint] = struct{}{}
	h.recentFingerprints[containerName] = append(h.recentFingerprints[containerName], fingerprint)
	if len(h.recentFingerprints[containerName]) > maxRecentLogEntries {
		toDrop := h.recentFingerprints[containerName][0]
		h.recentFingerprints[containerName] = h.recentFingerprints[containerName][1:]
		delete(h.recentSet[containerName], toDrop)
	}
}

// seedBoundaryFingerprints loads the lines already stored or sent to the master
// at exactly the cursor timestamp, so a replay of them is recognised while a
// genuinely new line sharing that nanosecond is still kept.
func (h *DaemonService) seedBoundaryFingerprints(host, containerName string, cursor time.Time) {
	if cursor.IsZero() {
		return
	}
	prefix := cursor.UTC().Format(streamTimestampFmt)
	sent := sentAtCursor(util.GetDB(host, containerName, "streamstate"), prefix)
	db := util.GetDB(host, containerName, "logs")

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	for _, fingerprint := range sent {
		h.rememberFingerprint(containerName, fingerprint)
	}
	if db == nil {
		return
	}
	iter := db.NewIterator(leveldbUtil.BytesPrefix([]byte(prefix)), nil)
	defer iter.Release()
	for iter.Next() {
		h.rememberFingerprint(containerName, prefix+" "+string(iter.Value()))
	}
}

func (h *DaemonService) countDroppedReplay(containerName string) {

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	h.droppedReplays[containerName]++
	h.droppedReplayTotals[util.GetHost()+"/"+containerName]++
	if count := h.droppedReplays[containerName]; count == 1 || count%100 == 0 {
		fmt.Printf("INFO: dropped %d replayed log line(s) for %s\n", count, containerName)
	}
}

func (h *DaemonService) DroppedReplays(containerName string) int {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	return h.droppedReplays[containerName]
}

// Keyed host/container, and survives a stream restart so the counter does not
// reset every time a container bounces.
func (h *DaemonService) DroppedReplayTotals() map[string]uint64 {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	return maps.Clone(h.droppedReplayTotals)
}

// Mirrors what saveCursor persisted, so a scrape needs no LevelDB.
func (h *DaemonService) CursorTimestamps() map[string]time.Time {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	return maps.Clone(h.cursorTimestamps)
}

func (h *DaemonService) isRecentDuplicate(containerName, fingerprint string) bool {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	if h.recentSet[containerName] == nil {
		h.recentSet[containerName] = map[string]struct{}{}
	}
	if _, exists := h.recentSet[containerName][fingerprint]; exists {
		return true
	}

	h.rememberFingerprint(containerName, fingerprint)
	return false
}

// storedThrough is the cursor as persisted: the timestamp of the newest line
// already stored. getResumeSince deliberately rewinds behind it, so everything
// the stream replays up to this point is already on disk.
func (h *DaemonService) storedThrough(host, containerName string) time.Time {
	db := util.GetDB(host, containerName, "streamstate")
	if db == nil {
		return time.Time{}
	}
	raw, err := db.Get([]byte(cursorKey), nil)
	if err != nil || len(raw) == 0 {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return time.Time{}
	}
	return ts
}

// ingestLine stores one streamed line, dropping anything the previous run had
// already persisted. Returns true when the line was stored.
func (h *DaemonService) ingestLine(host, containerName string, currentDB *leveldb.DB, token string, toHost bool, storedThrough time.Time, line string) bool {

	logItem, cursorTS, ok := parseDockerLogLine(line)
	if !ok {
		return false
	}

	// Docker's Since is floored to whole seconds and the cursor is rewound on
	// purpose, so every attach replays lines that are already stored.
	if !storedThrough.IsZero() && cursorTS.Before(storedThrough) {
		h.countDroppedReplay(containerName)
		return false
	}

	fingerprint := logItem[0] + " " + logItem[1]
	if h.isRecentDuplicate(containerName, fingerprint) {
		h.countDroppedReplay(containerName)
		return false
	}

	if toHost {
		agent.SendLogMessage(token, containerName, logItem)
		h.saveCursor(host, containerName, cursorTS, fingerprint)
		return true
	}

	if err := containerdb.PutLogMessage(currentDB, host, containerName, logItem); err != nil {
		fmt.Println("ERROR:", err.Error())
		return false
	}
	h.saveCursor(host, containerName, cursorTS, "")

	toSend, _ := json.Marshal(logItem)
	vars.Broadcast(containerName, websocket.TextMessage, toSend)
	return true
}

func (h *DaemonService) getResumeSince(host, containerName string) time.Time {
	db := util.GetDB(host, containerName, "streamstate")
	if db == nil {
		return time.Now().Add(-initialBackfill)
	}
	raw, err := db.Get([]byte(cursorKey), nil)
	if err != nil || len(raw) == 0 {
		return time.Now().Add(-initialBackfill)
	}

	ts, err := time.Parse(time.RFC3339Nano, string(raw))
	if err != nil {
		return time.Now().Add(-initialBackfill)
	}

	return ts.Add(-overlapBackfill)
}

func (h *DaemonService) saveCursor(host, containerName string, ts time.Time, sent string) {

	h.streamsMu.Lock()
	h.cursorTimestamps[host+"/"+containerName] = ts.UTC()
	h.streamsMu.Unlock()

	db := util.GetDB(host, containerName, "streamstate")
	if db == nil {
		return
	}
	cursor := ts.UTC().Format(streamTimestampFmt)
	batch := new(leveldb.Batch)
	batch.Put([]byte(cursorKey), []byte(cursor))
	if sent != "" {
		batch.Put([]byte(sentAtCursorKey), []byte(strings.Join(append(sentAtCursor(db, cursor), sent), "\n")))
	}
	if err := db.Write(batch, nil); err != nil {
		fmt.Println("ERROR: unable to save stream cursor:", err)
	}
}

func sentAtCursor(db *leveldb.DB, cursor string) []string {
	if db == nil {
		return nil
	}
	raw, err := db.Get([]byte(sentAtCursorKey), nil)
	if err != nil {
		return nil
	}
	return slices.DeleteFunc(strings.Split(string(raw), "\n"), func(fingerprint string) bool {
		return !strings.HasPrefix(fingerprint, cursor+" ")
	})
}

func scanLogs(ctx context.Context, reader io.Reader, onLine func(string)) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLogLine)
	discarding := false
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if discarding {
			i := bytes.IndexByte(data, '\n')
			if i < 0 {
				return len(data), nil, nil
			}
			discarding = false
			return i + 1, nil, nil
		}
		if len(data) >= maxLogLine && bytes.IndexByte(data, '\n') < 0 {
			discarding = true
			return len(data), append(data[:keptLongLine:keptLongLine], truncatedLineMark...), nil
		}
		return bufio.ScanLines(data, atEOF)
	})
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		onLine(scanner.Text())
	}

	if scanErr := scanner.Err(); scanErr != nil && !errors.Is(scanErr, io.EOF) {
		return scanErr
	}
	return nil
}

func (h *DaemonService) streamDockerLogs(ctx context.Context, rc io.Reader, onLine func(string), demux bool) error {
	if !demux {
		return scanLogs(ctx, rc, onLine)
	}

	pr, pw := io.Pipe()
	copyDone := make(chan error, 1)

	go func() {
		_, err := stdcopy.StdCopy(pw, pw, rc)
		copyDone <- err
		_ = pw.CloseWithError(err)
	}()

	scanErr := scanLogs(ctx, pr, onLine)

	// Release the writer before waiting on it. StdCopy may be parked in
	// pw.Write, and neither closing rc nor cancelling ctx unblocks a blocked
	// write -- so without this, an early return from scanLogs (cancellation)
	// leaves StdCopy stuck, this function never returns, finalizeStream never
	// runs, and EnsureStream refuses to restart that container for the life of
	// the process.
	_ = pr.CloseWithError(io.ErrClosedPipe)

	copyErr := <-copyDone
	if scanErr != nil {
		return scanErr
	}
	if copyErr != nil && !errors.Is(copyErr, io.EOF) && !errors.Is(copyErr, context.Canceled) &&
		!errors.Is(copyErr, io.ErrClosedPipe) {
		return copyErr
	}
	return nil
}

func (h *DaemonService) finalizeStream(containerName string, streamID uint64) (bool, context.Context) {
	if streamID == 0 {
		return true, nil
	}

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	currentID, exists := h.streamIDs[containerName]
	if !exists || currentID != streamID {
		return false, nil
	}
	restart := h.draining[containerName]
	delete(h.draining, containerName)

	// Dropping the CancelFunc without calling it leaks streamCtx and the
	// watchdog goroutine parked on <-ctx.Done() for every stream that ends.
	if cancel, ok := h.streamCancels[containerName]; ok {
		defer cancel(nil)
	}
	delete(h.streamCancels, containerName)
	delete(h.streamIDs, containerName)
	delete(h.recentSet, containerName)
	delete(h.recentFingerprints, containerName)
	delete(h.droppedReplays, containerName)
	// droppedReplayTotals deliberately survives; the cursor does not, since a
	// stopped stream has no progress to fall behind on.
	delete(h.cursorTimestamps, util.GetHost()+"/"+containerName)
	vars.RemoveActiveStream(containerName)
	return true, restart
}

func (h *DaemonService) endStream(containerName string, streamID uint64) {
	owned, restart := h.finalizeStream(containerName, streamID)
	if !owned {
		return
	}
	if restart != nil {
		h.EnsureStream(restart, containerName)
	}
}

func (h *DaemonService) runContainerStream(ctx context.Context, containerName string, toHost bool, streamID uint64) {
	host := util.GetHost()
	storedThrough := h.storedThrough(host, containerName)
	h.seedBoundaryFingerprints(host, containerName, storedThrough)
	since := h.getResumeSince(host, containerName)
	rc, err := h.DockerClient.Client.ContainerLogs(
		ctx,
		containerName,
		container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Follow:     true,
			Since:      strconv.FormatInt(since.Unix(), 10),
		},
	)
	if err != nil {
		fmt.Println("ERROR: unable to attach logs stream for", containerName, ":", err)
		h.endStream(containerName, streamID)
		return
	}
	defer rc.Close()

	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()

	currentDB := util.GetDB(host, containerName, "logs")
	token := os.Getenv("ONLOGS_TOKEN")
	noteMeta(currentDB, host, containerName, token, toHost, "ONLOGS: Container listening started!")

	// TTY streams start with a timestamp digit, multiplexed ones with a type byte (0-3).
	br := bufio.NewReader(rc)
	head, _ := br.Peek(1)
	demux := len(head) == 1 && stdcopy.StdType(head[0]) <= stdcopy.Systemerr

	streamErr := h.streamDockerLogs(ctx, br, func(line string) {
		h.ingestLine(host, containerName, currentDB, token, toHost, storedThrough, line)
	}, demux)

	if cause := context.Cause(ctx); cause == nil || cause == errDrainTimeout {
		if cause != nil {
			streamErr = cause
		}
		reason := "(EOF)"
		if streamErr != nil {
			reason = "(" + streamErr.Error() + ")"
		}
		noteMeta(currentDB, host, containerName, token, toHost, "ONLOGS: Container listening stopped! "+reason)
	}

	h.endStream(containerName, streamID)
}

func (h *DaemonService) EnsureStream(ctx context.Context, containerName string) {

	h.streamsMu.Lock()
	if _, exists := h.streamCancels[containerName]; exists {
		if _, draining := h.draining[containerName]; draining {
			h.draining[containerName] = ctx
			vars.AddActiveStream(containerName)
		}
		h.streamsMu.Unlock()
		return
	}
	streamCtx, cancel := context.WithCancelCause(ctx)
	h.streamSeq++
	streamID := h.streamSeq
	h.streamCancels[containerName] = cancel
	h.streamIDs[containerName] = streamID
	// The marker changes under streamsMu, with the ownership it reflects.
	vars.AddActiveStream(containerName)
	h.streamsMu.Unlock()

	if util.IsAgentMode() {
		go h.runContainerStream(streamCtx, containerName, true, streamID)
		return
	}
	go h.runContainerStream(streamCtx, containerName, false, streamID)
}

func (h *DaemonService) StopStream(containerName string) {

	h.streamsMu.Lock()
	cancel, exists := h.streamCancels[containerName]
	if _, draining := h.draining[containerName]; exists && !draining {
		h.draining[containerName] = nil
		time.AfterFunc(drainTimeout, func() { cancel(errDrainTimeout) })
	}
	vars.RemoveActiveStream(containerName)
	h.streamsMu.Unlock()
}

// runningNames keeps only containers docker reports as running. Attaching to an
// exited container writes a started/stopped META pair on every reconcile.
func runningNames(result []docker.ContainerNamesResult) []string {
	var names []string
	for i := range result {
		if result[i].Running && util.IsSafeName(result[i].Name) {
			names = append(names, result[i].Name)
		}
	}
	return names
}

// returns list of names of docker containers from docker daemon
func (h *DaemonService) GetContainersList(ctx context.Context) []string {
	result, err := h.DockerClient.GetContainerNames(ctx)
	if err != nil {
		fmt.Println("ERROR: failed to get containers list from docker daemon:", err)
		return vars.DockerContainerList()
	}

	var names []string

	containersMetaDB := util.GetContainersMetaDB(util.GetHost())
	if containersMetaDB == nil {
		return runningNames(result)
	}

	for i := range result {
		name := result[i].Name
		id := result[i].ID

		// Every container is recorded, so a stopped one can still be looked up
		// by name; only running ones are streamed and shown as enabled.
		containersMetaDB.Put([]byte(name), []byte(id), nil)
	}

	names = runningNames(result)
	return names
}
