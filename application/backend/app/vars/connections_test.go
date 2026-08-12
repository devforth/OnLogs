package vars

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func dialTestSocket(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

	served := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		served <- conn
	}))

	url := "ws" + server.URL[len("http"):]
	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial: %v", err)
	}

	serverConn := <-served
	return serverConn, func() {
		client.Close()
		serverConn.Close()
		server.Close()
	}
}

func TestConnectionsSurviveConcurrentRegistrationAndBroadcast(t *testing.T) {
	key := "racehost/racecontainer"
	conn, cleanup := dialTestSocket(t)
	defer cleanup()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// The websocket-upgrade handler registering viewers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			AddConnection(key+"-other", conn)
		}
	}()

	// The daemon goroutine fanning log lines out to viewers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			Broadcast(key, websocket.TextMessage, []byte("line"))
		}
	}()

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestBroadcastDropsAConnectionThatFailsToWrite(t *testing.T) {
	key := "deadhost/deadcontainer"
	conn, cleanup := dialTestSocket(t)
	defer cleanup()

	AddConnection(key, conn)
	if ConnectionCount(key) != 1 {
		t.Fatalf("expected the connection to be registered, got %d", ConnectionCount(key))
	}

	conn.Close()
	Broadcast(key, websocket.TextMessage, []byte("line"))

	if got := ConnectionCount(key); got != 0 {
		t.Fatalf("a dead connection was kept after a failed write: %d still registered", got)
	}
}

func TestAddConnectionIgnoresNil(t *testing.T) {
	key := "nilhost/nilcontainer"
	AddConnection(key, nil)

	if got := ConnectionCount(key); got != 0 {
		t.Fatalf("a nil connection was stored; a background goroutine will dereference it (%d stored)", got)
	}
	Broadcast(key, websocket.TextMessage, []byte("line"))
}
