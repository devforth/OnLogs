package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	cursorKey           = "last_cursor_ts"
	streamTimestampFmt  = "2006-01-02T15:04:05.000000000Z"
	overlapBackfill     = 2 * time.Second
	initialBackfill     = 30 * time.Second
	maxRecentLogEntries = 500
)

type DaemonService struct {
	DockerClient *docker.DockerService

	streamsMu          sync.Mutex
	streamCancels      map[string]context.CancelFunc
	streamIDs          map[string]uint64
	streamSeq          uint64
	recentFingerprints map[string][]string
	recentSet          map[string]map[string]struct{}
	droppedReplays     map[string]int
}

func (h *DaemonService) ensureRuntimeState() {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	if h.streamCancels == nil {
		h.streamCancels = map[string]context.CancelFunc{}
	}
	if h.streamIDs == nil {
		h.streamIDs = map[string]uint64{}
	}
	if h.recentFingerprints == nil {
		h.recentFingerprints = map[string][]string{}
	}
	if h.recentSet == nil {
		h.recentSet = map[string]map[string]struct{}{}
	}
	if h.droppedReplays == nil {
		h.droppedReplays = map[string]int{}
	}
}

func createLogMessage(db *leveldb.DB, host string, container string, message string) string {
	datetime := time.Now().UTC().Format(streamTimestampFmt)
	if db != nil {
		containerdb.PutLogMessage(db, host, container, []string{datetime, message})
	}
	return datetime + " " + message
}

func validateMessage(message string) (string, bool) {
	for !strings.HasPrefix(message, vars.Year) {
		message = message[1:]
		if len(message) < 31 {
			return "", false
		}
	}

	return message, true
}

func closeActiveStream(containerName string) {
	vars.RemoveActiveStream(containerName)
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

// seedBoundaryFingerprints loads the lines already stored at exactly the cursor
// timestamp, so a replay of them is recognised while a genuinely new line
// sharing that nanosecond is still kept.
func (h *DaemonService) seedBoundaryFingerprints(host, containerName string, cursor time.Time) {
	if cursor.IsZero() {
		return
	}
	db := util.GetDB(host, containerName, "logs")
	if db == nil {
		return
	}

	prefix := cursor.UTC().Format(streamTimestampFmt)
	iter := db.NewIterator(leveldbUtil.BytesPrefix([]byte(prefix)), nil)
	defer iter.Release()

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	for iter.Next() {
		h.rememberFingerprint(containerName, prefix+" "+string(iter.Value()))
	}
}

func (h *DaemonService) countDroppedReplay(containerName string) {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	h.droppedReplays[containerName]++
	if count := h.droppedReplays[containerName]; count == 1 || count%100 == 0 {
		fmt.Printf("INFO: dropped %d replayed log line(s) for %s\n", count, containerName)
	}
}

func (h *DaemonService) DroppedReplays(containerName string) int {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()
	return h.droppedReplays[containerName]
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
	h.ensureRuntimeState()

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
		h.saveCursor(host, containerName, cursorTS)
		return true
	}

	if err := containerdb.PutLogMessage(currentDB, host, containerName, logItem); err != nil {
		fmt.Println("ERROR:", err.Error())
		return false
	}
	h.saveCursor(host, containerName, cursorTS)

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

func (h *DaemonService) saveCursor(host, containerName string, ts time.Time) {
	db := util.GetDB(host, containerName, "streamstate")
	if db == nil {
		return
	}
	err := db.Put([]byte(cursorKey), []byte(ts.UTC().Format(streamTimestampFmt)), nil)
	if err != nil {
		fmt.Println("ERROR: unable to save stream cursor:", err)
	}
}

func scanLogs(ctx context.Context, reader io.Reader, onLine func(string)) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
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

func (h *DaemonService) streamDockerLogs(ctx context.Context, rc io.ReadCloser, onLine func(string), demux bool) error {
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
	// write -- so without this, an early return from scanLogs (cancellation, or
	// a line over the scanner's 1 MiB limit) leaves StdCopy stuck, this function
	// never returns, finalizeStream never runs, and EnsureStream refuses to
	// restart that container for the life of the process.
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

func (h *DaemonService) isContainerTTY(ctx context.Context, containerName string) bool {
	res, err := h.DockerClient.Client.ContainerInspect(ctx, containerName)
	if err != nil || res.Config == nil {
		return false
	}
	return res.Config.Tty
}

func (h *DaemonService) finalizeStream(containerName string, streamID uint64) bool {
	if streamID == 0 {
		return true
	}

	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	currentID, exists := h.streamIDs[containerName]
	if !exists || currentID != streamID {
		return false
	}

	// Dropping the CancelFunc without calling it leaks streamCtx and the
	// watchdog goroutine parked on <-ctx.Done() for every stream that ends.
	if cancel, ok := h.streamCancels[containerName]; ok {
		defer cancel()
	}
	delete(h.streamCancels, containerName)
	delete(h.streamIDs, containerName)
	delete(h.recentSet, containerName)
	delete(h.recentFingerprints, containerName)
	delete(h.droppedReplays, containerName)
	return true
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
		if h.finalizeStream(containerName, streamID) {
			closeActiveStream(containerName)
		}
		return
	}
	defer rc.Close()

	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()

	currentDB := util.GetDB(host, containerName, "logs")
	token := os.Getenv("ONLOGS_TOKEN")
	if toHost {
		agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening started!"), " ", 2))
	} else {
		createLogMessage(currentDB, host, containerName, "ONLOGS: Container listening started!")
	}

	streamErr := h.streamDockerLogs(ctx, rc, func(line string) {
		h.ingestLine(host, containerName, currentDB, token, toHost, storedThrough, line)
	}, !h.isContainerTTY(ctx, containerName))

	if streamErr != nil && ctx.Err() == nil {
		if toHost {
			agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening stopped! ("+streamErr.Error()+")"), " ", 2))
		} else {
			createLogMessage(currentDB, host, containerName, "ONLOGS: Container listening stopped! ("+streamErr.Error()+")")
		}
	} else if ctx.Err() == nil {
		if toHost {
			agent.SendLogMessage(token, containerName, strings.SplitN(createLogMessage(nil, host, containerName, "ONLOGS: Container listening stopped! (EOF)"), " ", 2))
		} else {
			createLogMessage(currentDB, host, containerName, "ONLOGS: Container listening stopped! (EOF)")
		}
	}

	if h.finalizeStream(containerName, streamID) {
		closeActiveStream(containerName)
	}
}

func (h *DaemonService) EnsureStream(ctx context.Context, containerName string) {
	h.ensureRuntimeState()

	h.streamsMu.Lock()
	if _, exists := h.streamCancels[containerName]; exists {
		h.streamsMu.Unlock()
		return
	}
	streamCtx, cancel := context.WithCancel(ctx)
	h.streamSeq++
	streamID := h.streamSeq
	h.streamCancels[containerName] = cancel
	h.streamIDs[containerName] = streamID
	h.streamsMu.Unlock()

	vars.AddActiveStream(containerName)

	if util.IsAgentMode() {
		go h.runContainerStream(streamCtx, containerName, true, streamID)
		return
	}
	go h.runContainerStream(streamCtx, containerName, false, streamID)
}

func (h *DaemonService) StopStream(containerName string) {
	h.ensureRuntimeState()

	h.streamsMu.Lock()
	cancel, exists := h.streamCancels[containerName]
	if exists {
		delete(h.streamCancels, containerName)
		delete(h.streamIDs, containerName)
	}
	h.streamsMu.Unlock()

	if exists {
		cancel()
	}
	closeActiveStream(containerName)
}

func (h *DaemonService) CreateDaemonToHostStream(ctx context.Context, containerName string) {
	h.runContainerStream(ctx, containerName, true, 0)
}

func (h *DaemonService) CreateDaemonToDBStream(ctx context.Context, containerName string) {
	h.runContainerStream(ctx, containerName, false, 0)
}

// runningNames keeps only containers docker reports as running. Attaching to an
// exited container writes a started/stopped META pair on every reconcile.
func runningNames(result []docker.ContainerNamesResult) []string {
	var names []string
	for i := range result {
		if result[i].Running && result[i].Name != "" {
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

func (h *DaemonService) GetContainerImageNameByContainerID(ctx context.Context, containerID string) string {
	result, err := h.DockerClient.GetContainerImageNameByContainerID(ctx, containerID)
	if err != nil {
		return ""
	}

	return result
}
