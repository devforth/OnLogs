package daemon

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
)

// The cursor is rewritten for every line and lives outside MAX_LOGS_SIZE.
func TestStreamCursorStaysSmall(t *testing.T) {
	t.Chdir(t.TempDir())
	host, name := util.GetHost(), "chatty"
	t.Cleanup(func() { util.ResetDB(host, name, "streamstate") })
	h := NewDaemonService(nil)
	start := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	for i := range 30000 {
		h.saveCursor(host, name, start.Add(time.Duration(i)*time.Millisecond), "")
	}

	var size int64
	_ = filepath.WalkDir("leveldb/hosts/"+host+"/containers/"+name+"/streamstate", func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	if size > 512<<10 {
		t.Errorf("the stream cursor takes %d bytes on disk", size)
	}
}
