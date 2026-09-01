package logging

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetupCreatesLogFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fleetamp.log")
	closeLog, err := Setup(Config{Level: "debug", Format: "json", File: path})
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	defer closeLog()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log file not created: %v", err)
	}
}

func TestParseLevelRejectsUnknownValue(t *testing.T) {
	if _, err := parseLevel("verbose"); err == nil {
		t.Fatal("expected unsupported level error")
	}
}
