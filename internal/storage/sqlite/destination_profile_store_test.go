package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func TestDestinationProfileStoreLifecycle(t *testing.T) {
	database, err := Open(context.Background(), filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := database.DestinationProfiles()
	profile := blueprints.NewDestinationProfile("Grafana", "Production", "otlphttp/grafana", "endpoint: https://example.invalid/otlp")
	if err := store.Create(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(context.Background(), profile.ID)
	if err != nil || got.Name != profile.Name || got.ExporterID != profile.ExporterID {
		t.Fatalf("unexpected stored profile: %#v, %v", got, err)
	}
	profiles, err := store.List(context.Background())
	if err != nil || len(profiles) != 1 {
		t.Fatalf("unexpected profile list: %#v, %v", profiles, err)
	}
	if err := store.Delete(context.Background(), profile.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), profile.ID); err != storage.ErrDestinationProfileNotFound {
		t.Fatalf("expected not found after deletion, got %v", err)
	}
}
