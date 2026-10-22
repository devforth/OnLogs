package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/devforth/OnLogs/app/util"
	"github.com/docker/docker/client"
)

func TestInitConfigGeneratesAJWTSecretWhenNoneIsConfigured(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", "")

	init_config()

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		t.Fatalf("JWT secret is %d bytes, want at least 32: %q", len(secret), secret)
	}

	persisted, err := os.ReadFile("leveldb/JWT_secret")
	if err != nil {
		t.Fatalf("secret was not persisted: %v", err)
	}
	if string(persisted) != secret {
		t.Fatalf("persisted secret %q does not match the active one %q", persisted, secret)
	}

	t.Setenv("JWT_SECRET", "")
	init_config()
	if os.Getenv("JWT_SECRET") != secret {
		t.Fatalf("second boot changed the secret: %q -> %q", secret, os.Getenv("JWT_SECRET"))
	}
}

func TestInitConfigReplacesAnEmptySecretFile(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", "")

	if err := os.MkdirAll("leveldb", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("leveldb/JWT_secret", nil, 0o600); err != nil {
		t.Fatal(err)
	}

	init_config()

	if len(os.Getenv("JWT_SECRET")) < 32 {
		t.Fatalf("empty secret file was accepted, JWT_SECRET=%q", os.Getenv("JWT_SECRET"))
	}
}

func TestInitConfigKeepsAnExplicitlyConfiguredSecret(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("JWT_SECRET", "configured-by-the-operator")

	init_config()

	if os.Getenv("JWT_SECRET") != "configured-by-the-operator" {
		t.Fatalf("configured secret was overwritten: %q", os.Getenv("JWT_SECRET"))
	}
	if _, err := os.Stat("leveldb/JWT_secret"); err == nil {
		t.Fatal("a configured secret must not be written to disk")
	}
}

func TestNewServerSetsAllTimeouts(t *testing.T) {
	s := newServer("2874", nil)

	if s.ReadHeaderTimeout == 0 {
		t.Error("ReadHeaderTimeout is unset, so a slowloris client can hold a connection open forever")
	}
	if s.ReadTimeout == 0 {
		t.Error("ReadTimeout is unset")
	}
	if s.WriteTimeout == 0 {
		t.Error("WriteTimeout is unset")
	}
	if s.IdleTimeout == 0 {
		t.Error("IdleTimeout is unset")
	}
}

func TestSigtermClosesTheDatabasesBeforeExiting(t *testing.T) {
	if dir := os.Getenv("ONLOGS_SIGTERM_DIR"); dir != "" {
		os.Chdir(dir)
		if err := util.GetDB("host", "app", "logs").Put([]byte("k"), []byte("v"), nil); err != nil {
			t.Fatal(err)
		}
		closeDatabasesOnSignal()
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
		time.Sleep(10 * time.Second)
		t.Fatal("still running after SIGTERM")
	}

	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestSigtermClosesTheDatabasesBeforeExiting$")
	cmd.Env = append(os.Environ(), "ONLOGS_SIGTERM_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if code := cmd.ProcessState.ExitCode(); code != 143 {
		t.Fatalf("SIGTERM ended the process with %d (%v), want 143\n%s", code, err, out)
	}
	logFile, err := os.ReadFile(dir + "/leveldb/hosts/host/containers/app/logs/LOG")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logFile), "db@close done") {
		t.Error("the logs database was left open at exit")
	}
}

func TestInitConfigRejectsAnUnparseableMaxLogsSize(t *testing.T) {
	if _, err := util.ParseHumanReadableSize("10GB"); err != nil {
		t.Fatalf("the default should parse: %v", err)
	}
	for _, value := range []string{"lots", "10 gigabytes", "GB", "-"} {
		if _, err := util.ParseHumanReadableSize(value); err == nil {
			t.Errorf("MAX_LOGS_SIZE=%q was accepted; retention would silently never run", value)
		}
	}
}

const ownID = "0123456789ab"

func dockerAt(t *testing.T, host string) *client.Client {
	cli, err := client.NewClientWithOpts(client.WithHost(host))
	if err != nil {
		t.Fatal(err)
	}
	return cli
}

func fakeDocker(t *testing.T, dockerName string) *client.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/info") && dockerName != "" {
			w.Write([]byte(`{"Name":"` + dockerName + `"}`))
			return
		}
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)
	return dockerAt(t, "tcp://"+srv.Listener.Addr().String())
}

func resolveOwnID(t *testing.T, cli *client.Client) (string, string) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	id := resolveHost(context.Background(), cli, ownID, ownID)
	os.Stdout = stdout
	w.Close()
	printed, _ := io.ReadAll(r)
	r.Close()
	return id, string(printed)
}

func TestHostIdIsTheDockerHostNameInsteadOfTheOwnContainerId(t *testing.T) {
	t.Setenv("ONLOGS_HOSTNAME", "")
	if got, _ := resolveOwnID(t, fakeDocker(t, "docker-host-01")); got != "docker-host-01" {
		t.Fatalf("host id is %q, want the Docker host name", got)
	}
}

func TestHostIdPrefersTheEnvironmentOverTheDockerHostName(t *testing.T) {
	t.Setenv("ONLOGS_HOSTNAME", "prod-1")
	if got, _ := resolveOwnID(t, fakeDocker(t, "docker-host-01")); got != "prod-1" {
		t.Fatalf("host id is %q, want ONLOGS_HOSTNAME", got)
	}
}

func TestHostIdUsesTheEnvironmentOrWarnsWhenDockerIsNotReachable(t *testing.T) {
	t.Chdir(t.TempDir())
	down := dockerAt(t, "unix://docker.sock")
	t.Setenv("ONLOGS_HOSTNAME", "prod-1")
	if got, _ := resolveOwnID(t, down); got != "prod-1" {
		t.Errorf("host id is %q, want ONLOGS_HOSTNAME", got)
	}
	t.Setenv("ONLOGS_HOSTNAME", "")
	if got, printed := resolveOwnID(t, down); got != ownID || !strings.Contains(printed, "WARNING") {
		t.Errorf("host id is %q and printed %q, want the container id and a warning", got, printed)
	}
}

func TestHostIdWarnsAboutAnUnusableEnvironmentName(t *testing.T) {
	t.Setenv("ONLOGS_HOSTNAME", "../etc")
	if got, printed := resolveOwnID(t, fakeDocker(t, "docker-host-01")); got != "docker-host-01" || !strings.Contains(printed, "WARNING") {
		t.Fatalf("host id is %q and printed %q, want the Docker host name and a warning", got, printed)
	}
}

func TestHostIdKeepsAMountedOrChosenHostname(t *testing.T) {
	t.Setenv("ONLOGS_HOSTNAME", "prod-1")
	for _, h := range [][2]string{{"docker-host-01", ownID}, {"abcdef012345", ownID}, {"webserver-01", "webserver-01"}} {
		if got := resolveHost(context.Background(), fakeDocker(t, "other"), h[0], h[1]); got != h[0] {
			t.Errorf("hostname %q became %q", h[0], got)
		}
	}
}

func TestHostIdStaysTheContainerIdWhenNoStableNameIsAvailable(t *testing.T) {
	t.Setenv("ONLOGS_HOSTNAME", "")
	if got, printed := resolveOwnID(t, fakeDocker(t, "")); got != ownID || !strings.Contains(printed, "WARNING") {
		t.Fatalf("host id is %q and printed %q, want the container id and a warning", got, printed)
	}
}
