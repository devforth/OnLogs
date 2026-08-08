package vars

import "testing"

func TestTotalConnections(t *testing.T) {
	connectionsMutex.Lock()
	connections = map[string][]*viewer{}
	connectionsMutex.Unlock()

	if got := TotalConnections(); got != 0 {
		t.Fatalf("empty: got %d, want 0", got)
	}

	first, closeFirst := dialTestSocket(t)
	defer closeFirst()
	second, closeSecond := dialTestSocket(t)
	defer closeSecond()

	AddConnection("container-a", first)
	if got := TotalConnections(); got != 1 {
		t.Errorf("one viewer: got %d, want 1", got)
	}

	// Across keys, not within one.
	AddConnection("host-b/container-b", second)
	if got := TotalConnections(); got != 2 {
		t.Errorf("two viewers on two keys: got %d, want 2", got)
	}

	connectionsMutex.Lock()
	connections = map[string][]*viewer{}
	connectionsMutex.Unlock()
}
