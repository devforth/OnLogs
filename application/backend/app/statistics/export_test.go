package statistics

var RestartStats = restartStats

func SetAfterStatsRead(hook func()) { afterStatsRead = hook }

// Workers left running by other tests read this hook under flushMu.
func SetBeforeStatsReset(hook func(location string)) {
	flushMu.Lock()
	defer flushMu.Unlock()
	beforeStatsReset = hook
}
