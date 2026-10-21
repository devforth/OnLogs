package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/client"
)

func TestGetContainerNamesIgnoresLinkAliases(t *testing.T) {
	listed := []map[string]any{
		{"Id": "1", "Names": []string{"/app/redis", "/redis"}, "State": "running"},
		{"Id": "2", "Names": []string{"/web/x", "/web2"}, "State": "running"},
		{"Id": "3", "Names": []string{"/web-db", "/web/web-db"}, "State": "running"},
		{"Id": "4", "Names": []string{"/app"}, "State": "exited"},
		{"Id": "5", "Names": []string{}, "State": "running"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/json") {
			json.NewEncoder(w).Encode(listed)
			return
		}
		w.Header().Set("Api-Version", "1.47")
		w.Write([]byte("OK"))
	}))
	t.Cleanup(srv.Close)

	cli, err := client.NewClientWithOpts(client.WithHost("tcp://"+strings.TrimPrefix(srv.URL, "http://")), client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cli.Close() })

	got, err := (&DockerService{Client: cli}).GetContainerNames(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []ContainerNamesResult{
		{Name: "redis", ID: "1", Running: true},
		{Name: "web2", ID: "2", Running: true},
		{Name: "web-db", ID: "3", Running: true},
		{Name: "app", ID: "4", Running: false},
		{Name: "", ID: "5", Running: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("container %s: got %+v, want %+v", want[i].ID, got[i], want[i])
		}
	}
}
