package vars

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const wsWriteDeadline = 10 * time.Second

// gorilla permits at most one concurrent writer per connection and panics
// otherwise, so each viewer carries its own write lock.
type viewer struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (v *viewer) write(messageType int, payload []byte) error {
	v.writeMu.Lock()
	defer v.writeMu.Unlock()

	v.conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
	return v.conn.WriteMessage(messageType, payload)
}

// AddConnection registers a viewer's websocket for a host/container.
func AddConnection(key string, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	connections[key] = append(connections[key], &viewer{conn: conn})
}

func viewersFor(key string) []*viewer {
	connectionsMutex.RLock()
	defer connectionsMutex.RUnlock()
	return append([]*viewer{}, connections[key]...)
}

func removeViewer(key string, target *viewer) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	remaining := connections[key][:0]
	for _, existing := range connections[key] {
		if existing != target {
			remaining = append(remaining, existing)
		}
	}
	if len(remaining) == 0 {
		delete(connections, key)
		return
	}
	connections[key] = remaining
}

// Broadcast writes to every viewer of a host/container, dropping any connection
// that errors or stalls. A viewer that never drains its socket would otherwise
// block the caller forever, which freezes ingestion for that container.
func Broadcast(key string, messageType int, payload []byte) {
	for _, v := range viewersFor(key) {
		if err := v.write(messageType, payload); err != nil {
			removeViewer(key, v)
			v.conn.Close()
		}
	}
}

func ConnectionCount(key string) int {
	connectionsMutex.RLock()
	defer connectionsMutex.RUnlock()
	return len(connections[key])
}
