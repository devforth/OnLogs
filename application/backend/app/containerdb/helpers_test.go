package containerdb

import (
	"strconv"
	"time"
)

var testYear = strconv.Itoa(time.Now().UTC().Year())

func fitsForSearch(logLine string, message string, caseSensetivity bool) bool {
	return fitsNormalizedSearch(logLine, normalizeForSearch(message, caseSensetivity), caseSensetivity)
}
