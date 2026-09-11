package main

import (
	"fmt"
	"io"
	"strings"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionRequested(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch strings.ToLower(args[0]) {
	case "--version", "-version", "version":
		return true
	default:
		return false
	}
}

func writeVersion(w io.Writer) {
	fmt.Fprintf(w, "FleetAMP %s (commit: %s, built: %s)\n", version, commit, buildDate)
}
