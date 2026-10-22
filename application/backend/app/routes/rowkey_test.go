package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/db"
	"github.com/devforth/OnLogs/app/statistics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/gorilla/websocket"
)

const sharedTimestamp = "2026-10-19T09:00:00.000000000Z"

func freshHost(t *testing.T, host, container string) {
	t.Helper()
	reset := func() {
		for _, kind := range []string{"logs", "statuses", "statistics"} {
			util.ResetDB(host, container, kind)
		}
		_ = os.RemoveAll("leveldb/hosts/" + host)
	}
	reset()
	t.Cleanup(reset)
}

func getLogWithPrev(t *testing.T, host, container, startWith string) [][]string {
	t.Helper()
	q := url.Values{"host": {host}, "id": {container}, "limit": {"10"}, "startWith": {startWith}}
	_, body := call(t, testCtrl.GetLogWithPrev, authedRequest(t, "testuser", "GET", "/api/v1/getLogWithPrev?"+q.Encode(), nil))
	var page struct {
		Logs [][]string `json:"logs"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("decoding %s: %v", body, err)
	}
	return page.Logs
}

func messagesOf(rows [][]string) []string {
	out := []string{}
	for _, row := range rows {
		out = append(out, row[1])
	}
	return out
}

func TestGetLogWithPrevOpensEveryRowOfABareTimestamp(t *testing.T) {
	host, container := "bareTimestampHost", "svc"
	freshHost(t, host, container)
	logs := util.GetDB(host, container, "logs")
	for _, line := range [][]string{
		{"2026-10-19T08:59:59.000000000Z", "ready"},
		{sharedTimestamp, "job 41"},
		{sharedTimestamp, "job 42"},
		{sharedTimestamp, "job 43"},
		{"2026-10-19T09:00:01.000000000Z", "idle"},
	} {
		if err := containerdb.PutLogMessage(logs, host, container, line); err != nil {
			t.Fatal(err)
		}
	}

	rows := getLogWithPrev(t, host, container, sharedTimestamp)
	if got, want := messagesOf(rows), []string{"job 43", "job 42", "job 41", "ready"}; !slices.Equal(got, want) {
		t.Fatalf("a link to a bare timestamp opened %q, want %q", got, want)
	}

	if got, want := messagesOf(getLogWithPrev(t, host, container, rows[1][2])), []string{"job 42", "job 41", "ready"}; !slices.Equal(got, want) {
		t.Fatalf("a link to a full key opened %q, want %q", got, want)
	}
}

func TestLiveLinesCarryTheirStorageKey(t *testing.T) {
	host, container := "liveKeyHost", "svc"
	freshHost(t, host, container)
	t.Cleanup(func() { statistics.StopWorker(host, container) })

	registered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testCtrl.GetLogsStream(w, r)
		close(registered)
	}))
	defer srv.Close()
	header := http.Header{"Cookie": {"onlogs-cookie=" + util.CreateJWT("testuser")}}
	conn, _, err := websocket.DefaultDialer.Dial("ws"+srv.URL[len("http"):]+"/?host="+host+"&id="+container, header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	<-registered

	token := db.CreateOnLogsToken()
	for _, msg := range []string{"job 51", "job 52"} {
		body, _ := json.Marshal(map[string]any{"Token": token, "Host": host, "Container": container, "LogLine": []string{sharedTimestamp, msg}})
		rr := httptest.NewRecorder()
		testCtrl.AddLogLine(rr, httptest.NewRequest("POST", "/api/v1/addLogLine", bytes.NewReader(body)))
		if rr.Code != http.StatusOK {
			t.Fatalf("addLogLine: status %d", rr.Code)
		}
	}

	sent := map[string]string{}
	for range 2 {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var frame []string
		if err := json.Unmarshal(raw, &frame); err != nil || len(frame) != 3 {
			t.Fatalf("live frame %s does not carry the storage key", raw)
		}
		sent[frame[1]] = frame[2]
	}

	stored := containerdb.GetLogs(false, false, host, container, "", 10, "", false, nil)["logs"].([][]string)
	if len(stored) != 2 {
		t.Fatalf("stored %d rows, want 2", len(stored))
	}
	for _, row := range stored {
		if sent[row[1]] != row[2] {
			t.Errorf("%q was sent with key %q, stored as %q", row[1], sent[row[1]], row[2])
		}
	}
}
