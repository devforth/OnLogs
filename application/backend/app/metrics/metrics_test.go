package metrics

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/containerdb"
	"github.com/devforth/OnLogs/app/util"
)

const testToken = "metrics-package-test-token"

func scrape(t *testing.T, method string, auth string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, "/api/v1/metrics", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rr := httptest.NewRecorder()
	Handler(nil).ServeHTTP(rr, req)
	return rr
}

func TestMetricsUnauthorized(t *testing.T) {
	os.Unsetenv("METRICS_TOKEN")

	rr := scrape(t, http.MethodGet, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
	if body := rr.Body.String(); body != "" {
		t.Errorf("body: got %q, want empty", body)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type: got %q, want unset", ct)
	}
}

func TestMetricsWrongToken(t *testing.T) {
	os.Setenv("METRICS_TOKEN", testToken)
	defer os.Unsetenv("METRICS_TOKEN")

	for _, auth := range []string{"Bearer wrong", "Bearer ", "", testToken, "Basic " + testToken} {
		rr := scrape(t, http.MethodGet, auth)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("auth %q: got %d, want %d", auth, rr.Code, http.StatusUnauthorized)
		}
	}
}

func TestMetricsAuthBeforeMethod(t *testing.T) {
	os.Setenv("METRICS_TOKEN", testToken)
	defer os.Unsetenv("METRICS_TOKEN")

	rr := scrape(t, http.MethodPost, "")
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated POST: got %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMetricsWrongMethod(t *testing.T) {
	os.Setenv("METRICS_TOKEN", testToken)
	defer os.Unsetenv("METRICS_TOKEN")

	rr := scrape(t, http.MethodPost, "Bearer "+testToken)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("authenticated POST: got %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestMetricsServesWithValidToken(t *testing.T) {
	os.Setenv("METRICS_TOKEN", testToken)
	defer os.Unsetenv("METRICS_TOKEN")

	rr := scrape(t, http.MethodGet, "Bearer "+testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != contentType {
		t.Errorf("Content-Type: got %q, want %q", ct, contentType)
	}
	if !strings.Contains(rr.Body.String(), "onlogs_build_info{version=") {
		t.Errorf("build info missing from:\n%s", rr.Body.String())
	}
}

// What promtool checks, without needing promtool: a HELP and a TYPE per family,
// and no series emitted twice.
func TestMetricsGoldenParses(t *testing.T) {
	os.Setenv("METRICS_TOKEN", testToken)
	defer os.Unsetenv("METRICS_TOKEN")

	host := util.GetHost()
	db := util.GetDB(host, "golden-container", "logs")
	if db == nil {
		t.Fatal("no logs db")
	}
	defer os.RemoveAll("leveldb/hosts/" + host + "/containers/golden-container")
	if err := containerdb.PutLogMessage(db, host, "golden-container", []string{
		"2026-08-08T10:00:00.000000000Z", "ERROR golden",
	}); err != nil {
		t.Fatalf("PutLogMessage: %v", err)
	}

	body := scrape(t, http.MethodGet, "Bearer "+testToken).Body.String()

	helped := map[string]bool{}
	typed := map[string]bool{}
	seen := map[string]bool{}

	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# HELP ") {
			helped[strings.Fields(line)[2]] = true
			continue
		}
		if strings.HasPrefix(line, "# TYPE ") {
			typed[strings.Fields(line)[2]] = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		series := line
		if i := strings.LastIndex(line, " "); i >= 0 {
			series = line[:i]
		}
		if seen[series] {
			t.Errorf("duplicate series: %s", series)
		}
		seen[series] = true

		name := series
		if i := strings.Index(series, "{"); i >= 0 {
			name = series[:i]
		}
		if !helped[name] {
			t.Errorf("%s has no # HELP before its first sample", name)
		}
		if !typed[name] {
			t.Errorf("%s has no # TYPE before its first sample", name)
		}
	}

	if len(seen) == 0 {
		t.Fatal("no samples emitted")
	}
}

// util.IsSafeName constrains container names today; the writer must not rely on it.
func TestLabelValuesAreEscaped(t *testing.T) {
	got := escapeLabelValue(`a"b\c` + "\n" + `d`)
	want := `a\"b\\c\nd`
	if got != want {
		t.Errorf("escapeLabelValue: got %q, want %q", got, want)
	}
}
