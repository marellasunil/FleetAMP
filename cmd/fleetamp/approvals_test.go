package main

import "testing"

func TestConfigurationLineDiffHighlightsChanges(t *testing.T) {
	rows := configurationLineDiff("receivers:\n  otlp:\nservice: {}\n", "receivers:\n  otlp:\nprocessors: {}\nservice: {}\n")
	if len(rows) != 4 {
		t.Fatalf("row count=%d, want 4: %#v", len(rows), rows)
	}
	if rows[0].Kind != "same" || rows[1].Kind != "same" || rows[2].Kind != "added" || rows[2].NewText != "processors: {}" || rows[3].Kind != "same" {
		t.Fatalf("unexpected diff: %#v", rows)
	}
}

func TestConfigurationLineDiffMarksRemoval(t *testing.T) {
	rows := configurationLineDiff("receivers: {}\nprocessors: {}\n", "receivers: {}\n")
	if len(rows) != 2 || rows[1].Kind != "removed" || rows[1].OldText != "processors: {}" {
		t.Fatalf("unexpected diff: %#v", rows)
	}
}
