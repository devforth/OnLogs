// Package metrics serves the Prometheus exposition. Nothing here may touch the
// filesystem or the Docker daemon: a scrape must not compete with ingestion.
package metrics

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
)

// Set at link time with -ldflags "-X .../app/metrics.Version=x.y.z".
var Version = "dev"

const contentType = "text/plain; version=0.0.4; charset=utf-8"

// Satisfied by *daemon.DaemonService. An interface so this package does not
// import app/daemon, and so rendering is testable without a docker stream.
type daemonState interface {
	DroppedReplayTotals() map[string]uint64
	CursorTimestamps() map[string]time.Time
}

// A nil ds is valid and omits the per-container ingestion series.
func Handler(ds daemonState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Before the method check: an unauthorized caller learns nothing.
		if !authorized(req) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if req.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		started := time.Now()
		w.Header().Set("Content-Type", contentType)
		write(w, ds)
		recordScrapeDuration(time.Since(started))
	}
}

// An unset METRICS_TOKEN is the feature being off, not an open endpoint.
func authorized(req *http.Request) bool {
	token := os.Getenv("METRICS_TOKEN")
	if token == "" {
		return false
	}

	offered := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if offered == req.Header.Get("Authorization") {
		return false // no "Bearer " prefix at all
	}
	return subtle.ConstantTimeCompare([]byte(offered), []byte(token)) == 1
}

func write(w io.Writer, ds daemonState) {
	writeLogLines(w)
	writeStreamUp(w)
	writeCursors(w, ds)
	writeDroppedReplays(w, ds)
	writeSizes(w)
	writeProcess(w)
	writeFilesystem(w)

	writeHeader(w, "onlogs_build_info", "gauge", "Build information, always 1.")
	fmt.Fprintf(w, "onlogs_build_info{version=\"%s\"} 1\n", escapeLabelValue(Version))
}

func sortedStrings(values []string) []string {
	sorted := append([]string{}, values...)
	sort.Strings(sorted)
	return sorted
}

func writeLogLines(w io.Writer) {
	counts := containerdb.LogLineCounts()
	if len(counts) == 0 {
		return
	}

	writeHeader(w, "onlogs_log_lines_total", "counter", "Log lines stored, by severity.")
	for _, location := range sortedKeys(counts) {
		host, container := splitLocation(location)
		for _, level := range sortedKeys(counts[location]) {
			fmt.Fprintf(w, "onlogs_log_lines_total{host=\"%s\",container=\"%s\",level=\"%s\"} %d\n",
				escapeLabelValue(host), escapeLabelValue(container), escapeLabelValue(level),
				counts[location][level])
		}
	}
}

func writeHeader(w io.Writer, name string, metricType string, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s %s\n", name, metricType)
}

// util.IsSafeName rejects a slash in either half, so the first one separates them.
func splitLocation(location string) (string, string) {
	host, container, found := strings.Cut(location, "/")
	if !found {
		return location, ""
	}
	return host, container
}

func escapeLabelValue(value string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(value)
}

// Sorted so a scrape is byte-for-byte stable between calls.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
