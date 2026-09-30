package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
)

func TestBlueprintCatalogStoresPatternsAndBlocks(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil { t.Fatal(err) }
	defer db.Close()
	store := db.DestinationProfiles()
	pattern := blueprints.NewPattern("Linux metrics", "", "linux", "hostmetrics", "collection_interval: 30s", []string{"metrics"})
	if err := store.CreatePattern(ctx, pattern); err != nil { t.Fatal(err) }
	block := blueprints.NewBlock("Memory limiter", "", "processors", "memory_limiter", "limit_mib: 512", []string{"metrics"}, []string{"linux"}, true, true)
	if err := store.CreateBlock(ctx, block); err != nil { t.Fatal(err) }
	patterns, err := store.ListPatterns(ctx)
	if err != nil || len(patterns) != 1 || patterns[0].ReceiverID != "hostmetrics" { t.Fatalf("patterns=%+v err=%v", patterns, err) }
	blocks, err := store.ListBlocks(ctx)
	if err != nil || len(blocks) != 1 || !blocks[0].Required || !blocks[0].Locked { t.Fatalf("blocks=%+v err=%v", blocks, err) }
}
