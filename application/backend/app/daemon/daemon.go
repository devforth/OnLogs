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
	"github.com/syndtr/goleveldb/leveldb"
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
	newDaemonStreams := make([]string, 0, len(vars.Active_Daemon_Streams))
	for _, stream := range vars.Active_Daemon_Streams {
		if stream != containerName {
			newDaemonStreams = append(newDaemonStreams, stream)
		}
	}
	vars.Active_Daemon_Streams = newDaemonStreams
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

func (h *DaemonService) isRecentDuplicate(containerName, fingerprint string) bool {
	h.streamsMu.Lock()
	defer h.streamsMu.Unlock()

	if h.recentSet[containerName] == nil {
		h.recentSet[containerName] = map[string]struct{}{}
	}
	if _, exists := h.recentSet[containerName][fingerprint]; exists {
		return true
	}

	h.recentSet[containerName][fingerprint] = struct{}{}
	h.recentFingerprints[containerName] = append(h.recentFingerprints[containerName], fingerprint)
	if len(h.recentFingerprints[containerName]) > maxRecentLogEntries {
		toDrop := h.recentFingerprints[containerName][0]
		h.recentFingerprints[containerName] = h.recentFingerprints[containerName][1:]
		delete(h.recentSet[containerName], toDrop)
	}

	return false
}

func (h *DaemonService) getResumeSince(host, containerName string) time.Time {
	db := util.GetDB(host, containerName, "streamstate")
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

	if scanErr := scanLogs(ctx, pr, onLine); scanErr != nil {
		return scanErr
	}

	copyErr := <-copyDone
	if copyErr != nil && !errors.Is(copyErr, io.EOF) && !errors.Is(copyErr, context.Canceled) {
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

	delete(h.streamCancels, containerName)
	delete(h.streamIDs, containerName)
	return true
}

func (h *DaemonService) runContainerStream(ctx context.Context, containerName string, toHost bool, streamID uint64) {
	host := util.GetHost()
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
		logItem, cursorTS, ok := parseDockerLogLine(line)
		if !ok {
			return
		}

		fingerprint := logItem[0] + " " + logItem[1]
		if h.isRecentDuplicate(containerName, fingerprint) {
			return
		}

		if toHost {
			agent.SendLogMessage(token, containerName, logItem)
			h.saveCursor(host, containerName, cursorTS)
			return
		}

		err := containerdb.PutLogMessage(currentDB, host, containerName, logItem)
		if err != nil {
			fmt.Println("ERROR:", err.Error())
			return
		}
		h.saveCursor(host, containerName, cursorTS)

		toSend, _ := json.Marshal(logItem)
		for _, c := range vars.Connections[containerName] {
			c.WriteMessage(1, toSend)
		}
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

	if !util.Contains(containerName, vars.Active_Daemon_Streams) {
		vars.Active_Daemon_Streams = append(vars.Active_Daemon_Streams, containerName)
	}

	if os.Getenv("AGENT") != "" {
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

// returns list of names of docker containers from docker daemon
func (h *DaemonService) GetContainersList(ctx context.Context) []string {
	result, err := h.DockerClient.GetContainerNames(ctx)
	if err != nil {
		fmt.Println("ERROR: failed to get containers list from docker daemon:", err)
		return vars.DockerContainers
	}

	var names []string

	containersMetaDB := vars.ContainersMeta_DBs[util.GetHost()]
	if containersMetaDB == nil {
		containersMetaDB, err := leveldb.OpenFile("leveldb/hosts/"+util.GetHost()+"/containersMeta", nil)
		if err != nil {
			panic(err)
		}
		vars.ContainersMeta_DBs[util.GetHost()] = containersMetaDB
	}
	containersMetaDB = vars.ContainersMeta_DBs[util.GetHost()]

	for i := range result {
		name := result[i].Name
		id := result[i].ID

		names = append(names, name)
		containersMetaDB.Put([]byte(name), []byte(id), nil)
	}

	return names
}

func (h *DaemonService) GetContainerImageNameByContainerID(ctx context.Context, containerID string) string {
	result, err := h.DockerClient.GetContainerImageNameByContainerID(ctx, containerID)
	if err != nil {
		return ""
	}

	return result
}
