package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestDriftPolicyDefaultsToReportOnlyAndPersists(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	store := database.DriftPolicy()
	policy, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if policy != configs.DriftPolicyReport {
		t.Fatalf("default policy = %q, want %q", policy, configs.DriftPolicyReport)
	}
	if err := store.Set(ctx, configs.DriftPolicyEnforce); err != nil {
		t.Fatal(err)
	}
	policy, err = store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if policy != configs.DriftPolicyEnforce {
		t.Fatalf("stored policy = %q, want %q", policy, configs.DriftPolicyEnforce)
	}
}
