// Tests in-memory immutable configuration artifact persistence.
package memory

import (
	"context"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestConfigStorePutGet(t *testing.T) {
	ctx := context.Background()
	store := NewConfigStore()
	config := configs.NewConfiguration("test.yaml", "1", "service: {}\n", "text/yaml")
	if err := store.Put(ctx, config); err != nil {
		t.Fatalf("put config: %v", err)
	}
	got, err := store.Get(ctx, config.ID)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if got.Hash != config.Hash || got.Content != config.Content {
		t.Fatalf("stored config mismatch")
	}
}
