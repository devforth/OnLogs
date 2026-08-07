package util

import (
	"os"
	"testing"
)

func TestIsAgentModeTreatsTheValueAsABoolean(t *testing.T) {
	cases := map[string]bool{
		"":         false,
		"false":    false,
		"FALSE":    false,
		"0":        false,
		"no":       false,
		"nonsense": false,
		"true":     true,
		"TRUE":     true,
		"1":        true,
	}

	for value, want := range cases {
		os.Setenv("AGENT", value)
		if got := IsAgentMode(); got != want {
			t.Errorf("AGENT=%q: agent mode is %v, want %v", value, got, want)
		}
	}
	os.Unsetenv("AGENT")
}

func TestParseHostnameHandlesAnEmptyFile(t *testing.T) {
	if host := parseHostname("", nil); host == "" {
		t.Error("an empty /etc/hostname produced an empty host, which indexes out of range downstream")
	}
	if host := parseHostname("\n", nil); host == "" {
		t.Error("a newline-only /etc/hostname produced an empty host")
	}
	if host := parseHostname("myhost\n", nil); host != "myhost" {
		t.Errorf("trailing newline not stripped: %q", host)
	}
	if host := parseHostname("myhost\r\n", nil); host != "myhost" {
		t.Errorf("trailing CRLF not stripped: %q", host)
	}
	if host := parseHostname("myhost", nil); host != "myhost" {
		t.Errorf("a clean hostname was altered: %q", host)
	}
}
