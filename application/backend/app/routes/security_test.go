package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// B1 — Frontend built the http.Dir ROOT from the raw URL including the query
// string, so `?x=/../../<path>` escaped the dist directory entirely and served
// any file on the filesystem to an unauthenticated caller.
func TestFrontendDoesNotServeFilesOutsideDist(t *testing.T) {
	ctrl := initTestConfig()
	os.MkdirAll("dist", 0o700)
	os.WriteFile("dist/index.html", []byte("text"), 0o600)

	os.MkdirAll("leveldb_probe", 0o700)
	os.WriteFile("leveldb_probe/JWT_secret", []byte("REAL-ONLOGS-JWT-SECRET"), 0o600)
	t.Cleanup(func() { os.RemoveAll("leveldb_probe") })

	for _, target := range []string{
		"/?x=/../../leveldb_probe/JWT_secret",
		"/index.html?x=/../../leveldb_probe/JWT_secret",
		"/?/../../leveldb_probe/JWT_secret",
	} {
		req, _ := http.NewRequest("GET", target, nil)
		rr := httptest.NewRecorder()
		http.HandlerFunc(ctrl.Frontend).ServeHTTP(rr, req)
		body, _ := io.ReadAll(rr.Result().Body)
		if strings.Contains(string(body), "REAL-ONLOGS-JWT-SECRET") {
			t.Fatalf("arbitrary file read through %q: %s", target, string(body))
		}
	}
}

// B14 — the prefix was stripped with ReplaceAll at every position, so any asset
// path that repeats the prefix was rewritten into a path that does not exist and
// silently fell back to index.html.
func TestFrontendStripsPathPrefixOnlyAtTheFront(t *testing.T) {
	ctrl := initTestConfig()
	t.Setenv("ONLOGS_PATH_PREFIX", "/logs")

	os.MkdirAll("dist/assets", 0o700)
	os.WriteFile("dist/index.html", []byte("text"), 0o600)
	os.WriteFile("dist/assets/logs-panel.js", []byte("PANEL_ASSET"), 0o600)
	t.Cleanup(func() { os.RemoveAll("dist/assets") })

	req, _ := http.NewRequest("GET", "/logs/assets/logs-panel.js", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(ctrl.Frontend).ServeHTTP(rr, req)
	body, _ := io.ReadAll(rr.Result().Body)
	if string(body) != "PANEL_ASSET" {
		t.Fatalf("expected the asset, got %q", string(body))
	}
}
