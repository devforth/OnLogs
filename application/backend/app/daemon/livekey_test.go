package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
)

func TestStoredLinesAreSentLiveWithTheirKey(t *testing.T) {
	host := util.GetHost()
	container := "LiveKey"
	reset := func() {
		util.ResetDB(host, container, "logs")
		util.ResetDB(host, container, "statuses")
		_ = os.RemoveAll("leveldb/hosts/" + host + "/containers/" + container)
	}
	reset()
	t.Cleanup(reset)

	registered := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil); err == nil {
			vars.AddConnection(container, "", "", conn)
		}
		close(registered)
	}))
	defer srv.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+srv.URL[len("http"):], nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	<-registered

	db := util.GetDB(host, container, "logs")
	ts := time.Date(2026, 10, 19, 9, 0, 5, 0, time.UTC).Format(time.RFC3339Nano)
	d := NewDaemonService(nil)
	for _, msg := range []string{"job 51", "job 52"} {
		if stored, _ := d.ingestLine(host, container, db, "", false, time.Time{}, ts+" "+msg); !stored {
			t.Fatalf("%q was not stored", msg)
		}
	}
	noteMeta(db, host, container, "", false, "ONLOGS: Container listening stopped! (EOF)")

	sent := map[string]bool{}
	for range 3 {
		client.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, raw, err := client.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var frame []string
		if err := json.Unmarshal(raw, &frame); err != nil || len(frame) != 3 {
			t.Fatalf("live frame %s does not carry the storage key", raw)
		}
		sent[frame[2]] = true
	}

	iter := db.NewIterator(nil, nil)
	defer iter.Release()
	for iter.Next() {
		if !sent[string(iter.Key())] {
			t.Errorf("stored key %q was never sent", iter.Key())
		}
	}
}
