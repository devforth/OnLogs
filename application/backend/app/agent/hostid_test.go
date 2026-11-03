package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devforth/OnLogs/app/util"
)

func TestAgentNamesItselfWithTheHostIdSetAtStartup(t *testing.T) {
	t.Chdir(t.TempDir())
	prev := util.GetHost()
	util.SetHost("stable-agent")
	t.Cleanup(func() { util.SetHost(prev) })

	hosts := make(chan string, 2)
	master := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Hostname, Host string }
		json.NewDecoder(r.Body).Decode(&body)
		hosts <- r.URL.Path + " " + body.Hostname + body.Host
	}))
	t.Cleanup(master.Close)
	t.Setenv("HOST", master.URL)

	if err := SendInitRequest(context.Background(), noServices); err != nil {
		t.Fatal(err)
	}
	SendLogMessage("token", "app", []string{"2026-10-20T11:13:32.000000000Z", "line"})
	for _, want := range []string{"/api/v1/addHost stable-agent", "/api/v1/addLogLine stable-agent"} {
		if got := <-hosts; got != want {
			t.Errorf("master got %q, want %q", got, want)
		}
	}
}
