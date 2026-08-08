package hostalias

import (
	"strings"
	"testing"

	"github.com/devforth/OnLogs/app/vars"
)

func TestValidateRejectsUnsafeInput(t *testing.T) {
	cases := []struct {
		label   string
		host    string
		alias   string
		wantErr bool
	}{
		{"plain", "myhost", "prod box", false},
		{"clearing", "myhost", "", false},
		{"exactly 64 bytes", "myhost", strings.Repeat("a", 64), false},
		{"65 bytes", "myhost", strings.Repeat("a", 65), true},
		{"NUL in alias", "myhost", "prod\x00box", true},
		{"leading space", "myhost", " prod", true},
		{"trailing space", "myhost", "prod ", true},
		{"host escaping its directory", "..", "prod", true},
		{"host with a separator", "a/b", "prod", true},
		{"empty host", "", "prod", true},
	}

	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			err := Validate(c.host, c.alias)
			if c.wantErr && err == nil {
				t.Fatalf("Validate(%q, %q) = nil, want an error", c.host, c.alias)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("Validate(%q, %q) = %v, want nil", c.host, c.alias, err)
			}
		})
	}
}

func TestSetAndClearRoundTrip(t *testing.T) {
	const host = "alias-round-trip-host"
	t.Cleanup(func() { vars.AliasDB.Delete([]byte(host), nil) })

	if err := Set(host, "prod box"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	aliases, err := All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if aliases[host] != "prod box" {
		t.Fatalf("All()[%q] = %q, want %q", host, aliases[host], "prod box")
	}

	if err := Set(host, ""); err != nil {
		t.Fatalf("clearing: %v", err)
	}
	aliases, _ = All()
	if _, present := aliases[host]; present {
		t.Fatalf("the alias survived being cleared: %q", aliases[host])
	}
}

func TestSetRejectsInvalidWithoutWriting(t *testing.T) {
	const host = "alias-reject-host"
	t.Cleanup(func() { vars.AliasDB.Delete([]byte(host), nil) })

	if err := Set(host, strings.Repeat("a", 65)); err == nil {
		t.Error("Set accepted an oversized display name")
	}
	if aliases, _ := All(); aliases[host] != "" {
		t.Errorf("a rejected alias was stored anyway: %q", aliases[host])
	}
}
