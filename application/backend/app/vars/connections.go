package vars

import (
	"time"

	"github.com/gorilla/websocket"
)

const wsWriteDeadline = 10 * time.Second

// AddConnection registers a viewer's websocket for a host/container.
func AddConnection(key string, conn *websocket.Conn) {
	if conn == nil {
		return
	}
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()
	Connections[key] = append(Connections[key], conn)
}

func connectionsFor(key string) []*websocket.Conn {
	connectionsMutex.RLock()
	defer connectionsMutex.RUnlock()
	return append([]*websocket.Conn{}, Connections[key]...)
}

func removeConnection(key string, conn *websocket.Conn) {
	connectionsMutex.Lock()
	defer connectionsMutex.Unlock()

	remaining := Connections[key][:0]
	for _, existing := range Connections[key] {
		if existing != conn {
			remaining = append(remaining, existing)
		}
	}
	if len(remaining) == 0 {
		delete(Connections, key)
		return
	}
	Connections[key] = remaining
}

// Broadcast writes to every viewer of a host/container, dropping any connection
// that errors or stalls. A viewer that never drains its socket would otherwise
// block the caller forever, which freezes ingestion for that container.
func Broadcast(key string, messageType int, payload []byte) {
	for _, conn := range connectionsFor(key) {
		conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
		if err := conn.WriteMessage(messageType, payload); err != nil {
			removeConnection(key, conn)
			conn.Close()
		}
	}
}

func ConnectionCount(key string) int {
	connectionsMutex.RLock()
	defer connectionsMutex.RUnlock()
	return len(Connections[key])
}
