package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestLoadAgentSnapshotQuarantinesInvalidContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "truncated", content: `[{"instance_uid":"collector-1"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			path := filepath.Join(dataDir, "agents.json")
			if err := os.WriteFile(path, []byte(test.content), 0o640); err != nil {
				t.Fatal(err)
			}
			store := memory.NewAgentStore()
			if err := loadAgentSnapshot(context.Background(), store, dataDir); err != nil {
				t.Fatalf("load recovered snapshot: %v", err)
			}
			items, err := store.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 0 {
				t.Fatalf("recovered inventory contains %d agents", len(items))
			}
			replacement, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(replacement) != "[]\n" {
				t.Fatalf("replacement=%q, want valid empty JSON array", replacement)
			}
			backups, err := filepath.Glob(filepath.Join(dataDir, "agents.json.corrupt-*"))
			if err != nil || len(backups) != 1 {
				t.Fatalf("backups=%v err=%v, want one forensic backup", backups, err)
			}
			preserved, err := os.ReadFile(backups[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(preserved) != test.content {
				t.Fatalf("backup=%q, want original %q", preserved, test.content)
			}
		})
	}
}
