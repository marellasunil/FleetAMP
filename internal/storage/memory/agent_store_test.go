// Tests concurrency-safe in-memory ManagedAgent persistence behavior.
package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func TestAgentStoreUpsertGetListDelete(t *testing.T) {
	ctx := context.Background()
	store := NewAgentStore()
	agent := &agents.ManagedAgent{InstanceUID: "agent-1", Name: "otel-gateway", Connected: true}

	if err := store.Upsert(ctx, agent); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "agent-1")
	if err != nil || got.Name != "otel-gateway" {
		t.Fatalf("unexpected get result: agent=%v err=%v", got, err)
	}

	list, err := store.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("unexpected list result: len=%d err=%v", len(list), err)
	}
	if err := store.Delete(ctx, "agent-1"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Get(ctx, "agent-1")
	if !errors.Is(err, storage.ErrAgentNotFound) {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}
}

func TestAgentStorePreservesDisplayNameAcrossProtocolUpdates(t *testing.T) {
	ctx := context.Background()
	store := NewAgentStore()
	if err := store.Upsert(ctx, &agents.ManagedAgent{
		InstanceUID: "agent-1", Name: "payments-prod-collector-01",
		DisplayName: "payments-prod-collector-01",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(ctx, &agents.ManagedAgent{
		InstanceUID: "agent-1", Name: "reported-hostname", Connected: true,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "agent-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "payments-prod-collector-01" || got.Name != got.DisplayName {
		t.Fatalf("display name was not preserved: %#v", got)
	}
}
