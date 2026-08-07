package containerdb

import "testing"

// Shared with frontend/src/Views/Logs/classifier.test.mjs: both implementations
// must agree on every one of these.
var classifierCases = []struct {
	line  string
	level string
}{
	{"ERROR something failed", "error"},
	{"Error: connection refused", "error"},
	{"error: connection refused", "error"},
	{"ERR disk full", "error"},
	{"WARN low memory", "warn"},
	{"WARNING low memory", "warn"},
	{"warning low memory", "warn"},
	{"DEBUG entering loop", "debug"},
	{"debug entering loop", "debug"},
	{"INFO started", "info"},
	{"INFO ERROR_COUNT=0", "info"},
	{"ONLOGS: Container listening started!", "meta"},
	{"plain application output", "other"},
	{"", "other"},
	{"\x1b[31mERROR\x1b[0m red text", "error"},
	{"the word information appears here", "info"},
	{"2026-02-10 INFO ready", "info"},
}

func TestGetLogStatusKeyMatchesTheSharedRules(t *testing.T) {
	for _, c := range classifierCases {
		if got := GetLogStatusKey(c.line); got != c.level {
			t.Errorf("GetLogStatusKey(%q) = %q, want %q", c.line, got, c.level)
		}
	}
}
