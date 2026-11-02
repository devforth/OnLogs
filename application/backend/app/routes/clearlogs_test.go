package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
)

// The UI reads the size and the rows right after this answers.
func TestClearLogsAnswersAfterTheRowsAreGone(t *testing.T) {
	host, container := "clearLogsHost", "svc"
	freshHost(t, host, container)
	logs := util.GetDB(host, container, "logs")
	base := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 200; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format("2006-01-02T15:04:05.000000000Z07:00")
		if err := containerdb.PutLogMessage(logs, host, container, []string{ts, "INFO line"}); err != nil {
			t.Fatal(err)
		}
	}

	req := authedRequest(t, "admin", "POST", "/api/v1/deleteContainerLogs", map[string]string{"host": host, "service": container})
	if status, body := call(t, testCtrl.DeleteContainerLogs, req); status != http.StatusOK {
		t.Fatalf("deleteContainerLogs: %d %s", status, body)
	}

	iter := util.GetDB(host, container, "logs").NewIterator(nil, nil)
	defer iter.Release()
	left := 0
	for iter.Next() {
		left++
	}
	if left != 0 {
		t.Fatalf("%d rows were still stored when Clear logs answered", left)
	}
}

// The dialog shows the refusal only when it is a string.
func TestDeleteDockerLogsExplainsARefusal(t *testing.T) {
	vars.SetDockerContainers([]string{"busy"}, vars.ContainerEventSeq())
	t.Cleanup(func() { vars.SetDockerContainers(nil, vars.ContainerEventSeq()) })

	req := authedRequest(t, "admin", "POST", "/api/v1/deleteDockerLogs", map[string]string{"host": util.GetHost(), "service": "busy"})
	_, body := call(t, testCtrl.DeleteDockerLogs, req)
	var reply struct{ Error any }
	if err := json.Unmarshal(body, &reply); err != nil {
		t.Fatal(err)
	}
	if msg, _ := reply.Error.(string); !strings.Contains(msg, "Stop the container") {
		t.Errorf("deleteDockerLogs of a running container answered %s", body)
	}
}
