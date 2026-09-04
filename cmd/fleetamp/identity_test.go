package main

import (
	"context"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestInheritManagedMetadataByStableID(t *testing.T) {
	store := memory.NewAgentStore()
	previous := &agents.ManagedAgent{
		InstanceUID: "old-uid",
		Type:        agents.AgentTypeOTelCollector,
		Status:      agents.LifecycleDisconnected,
		Attributes:  map[string]string{stableAgentIDAttribute: "gateway-eu"},
		GroupFields: map[string]string{"application": "payments", "environment": "test", "place": "laptop"},
		Labels:      map[string]string{"owner": "observability"},
	}
	if err := store.Upsert(context.Background(), previous); err != nil {
		t.Fatal(err)
	}
	incoming := &agents.ManagedAgent{
		InstanceUID: "new-uid",
		Type:        agents.AgentTypeOTelCollector,
		Connected:   true,
		Attributes:  map[string]string{stableAgentIDAttribute: "gateway-eu"},
	}
	source, ok := inheritManagedMetadataByStableID(context.Background(), store, incoming)
	if !ok || source != "old-uid" {
		t.Fatalf("source=%q inherited=%t", source, ok)
	}
	if incoming.GroupFields["application"] != "payments" {
		t.Fatalf("group fields not inherited: %#v", incoming.GroupFields)
	}
	if incoming.Labels["owner"] != "observability" {
		t.Fatalf("labels not inherited: %#v", incoming.Labels)
	}
}

func TestStableIDReassociationRejectsAmbiguousOrActiveMatches(t *testing.T) {
	tests := []struct {
		name   string
		agents []*agents.ManagedAgent
	}{
		{"active", []*agents.ManagedAgent{
			stableIdentityAgent("old-1", true),
		}},
		{"ambiguous", []*agents.ManagedAgent{
			stableIdentityAgent("old-1", false),
			stableIdentityAgent("old-2", false),
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memory.NewAgentStore()
			for _, item := range tt.agents {
				if err := store.Upsert(context.Background(), item); err != nil {
					t.Fatal(err)
				}
			}
			incoming := stableIdentityAgent("new-uid", true)
			if source, ok := inheritManagedMetadataByStableID(context.Background(), store, incoming); ok {
				t.Fatalf("unexpected reassociation from %q", source)
			}
		})
	}
}

func TestStableIDIsRequiredForReassociation(t *testing.T) {
	store := memory.NewAgentStore()
	if err := store.Upsert(context.Background(), stableIdentityAgent("old-uid", false)); err != nil {
		t.Fatal(err)
	}
	incoming := &agents.ManagedAgent{
		InstanceUID: "new-uid",
		Type:        agents.AgentTypeOTelCollector,
		Attributes:  map[string]string{"service.name": "otelcol"},
	}
	if _, ok := inheritManagedMetadataByStableID(context.Background(), store, incoming); ok {
		t.Fatal("reassociated without explicit stable identity")
	}
}

func stableIdentityAgent(uid string, connected bool) *agents.ManagedAgent {
	return &agents.ManagedAgent{
		InstanceUID: uid,
		Type:        agents.AgentTypeOTelCollector,
		Connected:   connected,
		Status:      agents.LifecycleDisconnected,
		Attributes:  map[string]string{stableAgentIDAttribute: "gateway-eu"},
		GroupFields: map[string]string{"application": "payments"},
	}
}
