package routes

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/metrics"
	"github.com/devforth/OnLogs/app/util"
	"github.com/devforth/OnLogs/app/vars"
	"github.com/gorilla/websocket"
)

func waitForViewers(t *testing.T, container string, want int) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); vars.ConnectionCount(container) != want; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s has %d viewers, want %d", container, vars.ConnectionCount(container), want)
		}
	}
}

func viewerMetric(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer stream-metrics")
	rr := httptest.NewRecorder()
	metrics.Handler(nil).ServeHTTP(rr, req)
	for _, line := range strings.Split(rr.Body.String(), "\n") {
		if strings.HasPrefix(line, "onlogs_websocket_connections ") {
			return line
		}
	}
	t.Fatalf("no viewer gauge in metrics (status %d): %s", rr.Code, rr.Body.String())
	return ""
}

func TestClosedViewersOfAQuietContainerAreUnregistered(t *testing.T) {
	t.Setenv("METRICS_TOKEN", "stream-metrics")
	server := httptest.NewServer(http.HandlerFunc(testCtrl.GetLogsStream))
	defer server.Close()

	container := "quiet-closed-viewers"
	baseline := vars.TotalConnections()
	baselineMetric := viewerMetric(t)
	cookie := util.CreateJWT("admin")

	var clients []*websocket.Conn
	for i := 1; i <= 4; i++ {
		clients = append(clients, openStream(t, server, container, cookie))
		waitForViewers(t, container, i)
	}
	for _, client := range clients {
		client.Close()
	}

	waitForViewers(t, container, 0)
	if got := vars.TotalConnections(); got != baseline {
		t.Errorf("total viewers %d, want %d", got, baseline)
	}
	if got := viewerMetric(t); got != baselineMetric {
		t.Errorf("metric %q, want %q", got, baselineMetric)
	}
}

func TestViewerClosingWithACloseFrameIsUnregisteredAndClosed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(testCtrl.GetLogsStream))
	defer server.Close()

	container := "close-frame-viewer"
	cookie := util.CreateJWT("admin")
	kept := openStream(t, server, container, cookie)
	closing := openStream(t, server, container, cookie)
	waitForViewers(t, container, 2)

	closing.SetCloseHandler(func(int, string) error { return nil })
	if err := closing.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
		t.Fatal(err)
	}
	closing.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := closing.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatalf("the server did not answer the close frame: %v", err)
	}
	if _, err := closing.NetConn().Read(make([]byte, 1)); err != io.EOF {
		t.Errorf("the server kept its side open: %v", err)
	}
	waitForViewers(t, container, 1)

	vars.Broadcast(container, websocket.TextMessage, []byte("still live"))
	kept.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, msg, err := kept.ReadMessage(); err != nil || string(msg) != "still live" {
		t.Fatalf("the other viewer got %q, %v", msg, err)
	}
}

func waitForKeepAlives(t *testing.T, want int) {
	t.Helper()
	count := func() int {
		buf := make([]byte, 1<<22)
		return strings.Count(string(buf[:runtime.Stack(buf, true)]), "routes.keepAlive(")
	}
	for deadline := time.Now().Add(2 * time.Second); count() != want; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%d keepalive goroutines, want %d", count(), want)
		}
	}
}

func TestAQuietStreamIsPingedUntilTheViewerLeaves(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(testCtrl.GetLogsStream))
	defer server.Close()

	container := "quiet-pinged-viewer"
	waitForKeepAlives(t, 0)
	client := openStream(t, server, container, util.CreateJWT("admin"))
	waitForViewers(t, container, 1)

	// Lines keep arriving meanwhile so the race detector sees pings beside writes.
	stop := make(chan struct{})
	broadcasting := make(chan struct{})
	go func() {
		defer close(broadcasting)
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
				vars.Broadcast(container, websocket.TextMessage, []byte("line"))
			}
		}
	}()

	pinged := errors.New("pinged")
	pings := 0
	client.SetPingHandler(func(string) error {
		if pings++; pings == 3 {
			return pinged
		}
		return nil
	})
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	var err error
	for err == nil {
		_, _, err = client.ReadMessage()
	}
	close(stop)
	<-broadcasting
	if !errors.Is(err, pinged) {
		t.Fatalf("got %d pings in 2s: %v", pings, err)
	}

	client.Close()
	waitForViewers(t, container, 0)
	waitForKeepAlives(t, 0)
}
