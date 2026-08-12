package metrics

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

// Long because producing it walks every stored container.
const sizeRefreshInterval = 5 * time.Minute

type sizeSnapshot struct {
	perContainer map[string]int64
	total        int64
}

// nil until the first refresh, so a scrape during startup omits the size series
// rather than reporting an empty disk.
var sizes atomic.Pointer[sizeSnapshot]

func setSizes(snapshot *sizeSnapshot) { sizes.Store(snapshot) }

// RefreshSizes performs one walk.
func RefreshSizes() {
	perContainer, total := util.LogsSizes()
	setSizes(&sizeSnapshot{perContainer: perContainer, total: total})
}

// Deliberately its own ticker, not the streamer's 60s reconcile loop: too
// frequent for a full walk, and that loop already carries an unsynchronised write.
func StartSizeRefresher(ctx context.Context) {
	RefreshSizes()

	ticker := time.NewTicker(sizeRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			RefreshSizes()
		}
	}
}

func writeSizes(w io.Writer) {
	if snapshot := sizes.Load(); snapshot != nil {
		writeHeader(w, "onlogs_logs_size_bytes", "gauge",
			"Total size of stored logs, the number the retention quota is compared against.")
		fmt.Fprintf(w, "onlogs_logs_size_bytes %d\n", snapshot.total)

		if len(snapshot.perContainer) > 0 {
			writeHeader(w, "onlogs_container_db_size_bytes", "gauge", "Stored log size per container.")
			for _, location := range sortedKeys(snapshot.perContainer) {
				host, container := splitLocation(location)
				fmt.Fprintf(w, "onlogs_container_db_size_bytes{host=\"%s\",container=\"%s\"} %d\n",
					escapeLabelValue(host), escapeLabelValue(container), snapshot.perContainer[location])
			}
		}
	}

	// Omitted rather than guessed if it does not parse.
	if limit, err := util.ParseHumanReadableSize(os.Getenv("MAX_LOGS_SIZE")); err == nil {
		writeHeader(w, "onlogs_logs_size_limit_bytes", "gauge", "MAX_LOGS_SIZE in bytes.")
		fmt.Fprintf(w, "onlogs_logs_size_limit_bytes %d\n", limit)
	}

	writeHeader(w, "onlogs_retention_deleted_lines_total", "counter",
		"Log lines deleted by the size quota.")
	fmt.Fprintf(w, "onlogs_retention_deleted_lines_total %d\n", containerdb.RetentionDeletedLines())
}
