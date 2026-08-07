package main

import (
	"os"
	"testing"

	"github.com/devforth/OnLogs/app/util"
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

	// A second boot must reuse the persisted key.
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
