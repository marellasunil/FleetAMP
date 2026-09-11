package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionRequested(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-version"}, {"version"}} {
		if !versionRequested(args) {
			t.Fatalf("versionRequested(%q) = false", args)
		}
	}
	for _, args := range [][]string{nil, {}, {"--help"}, {"--version", "extra"}} {
		if versionRequested(args) {
			t.Fatalf("versionRequested(%q) = true", args)
		}
	}
}

func TestWriteVersion(t *testing.T) {
	var output bytes.Buffer
	writeVersion(&output)
	for _, expected := range []string{"FleetAMP", version, commit, buildDate} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("version output %q does not contain %q", output.String(), expected)
		}
	}
}
