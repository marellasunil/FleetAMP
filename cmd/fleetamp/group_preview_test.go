package main

import (
	"context"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestValidateConfigurationForApproval(t *testing.T) {
	validator := configs.NewValidator("")
	valid := configs.NewConfiguration("gateway.yaml", "1", "receivers:\n  otlp:\nservice:\n  pipelines: {}\n", "text/yaml")
	if err := validateConfigurationForApproval(context.Background(), validator, valid); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}

	invalid := configs.NewConfiguration("gateway.yaml", "2", "service:\n  pipelines: [\n", "text/yaml")
	if err := validateConfigurationForApproval(context.Background(), validator, invalid); err == nil {
		t.Fatal("invalid configuration accepted for approval")
	}
}

func TestNormalizeOwnersAndOwnershipMatch(t *testing.T) {
	owners := normalizeOwners([]string{" Alice, bob ", "alice", "BOB", "carol"})
	if len(owners) != 3 || owners[0] != "Alice" || owners[1] != "bob" || owners[2] != "carol" {
		t.Fatalf("owners=%#v", owners)
	}
	group := &groups.Group{Owners: owners}
	if !isGroupOwner(group, "ALICE") || isGroupOwner(group, "mallory") {
		t.Fatalf("unexpected ownership match for %#v", owners)
	}
}

func TestPreviewGroupMembers(t *testing.T) {
	members := []*agents.ManagedAgent{
		{InstanceUID: "ready", Connected: true, Capabilities: []string{"accepts_remote_config"}},
		{InstanceUID: "offline", Connected: false, Capabilities: []string{"accepts_remote_config"}},
		{InstanceUID: "unsupported", Connected: true},
		{InstanceUID: "retired", Connected: false, Status: agents.LifecycleRetired},
	}
	result, eligible := previewGroupMembers(members, true)
	if eligible != 1 {
		t.Fatalf("eligible = %d, want 1", eligible)
	}
	want := []string{"Ready", "Offline", "Remote configuration unsupported", "Retired"}
	for i, agent := range result {
		if agent.Agent.InstanceUID != members[i].InstanceUID || agent.Reason != want[i] {
			t.Errorf("result[%d] = %+v, want %s", i, agent, want[i])
		}
	}
	result, eligible = previewGroupMembers(members, false)
	if eligible != 0 {
		t.Fatalf("disabled group eligible = %d, want 0", eligible)
	}
	for _, agent := range result {
		if agent.Reason != "Group disabled" {
			t.Errorf("disabled group member %q: %q", agent.Agent.InstanceUID, agent.Reason)
		}
	}
}

func TestPreviewGroupConfigurationSkipsCurrentButAllowsOlderVersion(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAssignmentStore()
	agent := &agents.ManagedAgent{
		InstanceUID: "agent-1", Connected: true,
		Capabilities: []string{"accepts_remote_config"},
	}
	current := configs.NewConfiguration("collector.yaml", "2", "service: {pipelines: {}}", "text/yaml")
	older := configs.NewConfiguration("collector.yaml", "1", "service: {}", "text/yaml")
	if err := store.Upsert(ctx, &configs.Assignment{
		AgentInstanceUID: agent.InstanceUID, ConfigurationID: current.ID,
		ConfigurationHash: current.Hash, Status: configs.DeliveryApplied, UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	preview, eligible, err := previewGroupConfiguration(ctx, []*agents.ManagedAgent{agent}, true, current, store)
	if err != nil {
		t.Fatal(err)
	}
	if eligible != 0 || preview[0].Reason != "Already deployed · Latest" {
		t.Fatalf("current preview=%+v eligible=%d", preview, eligible)
	}

	preview, eligible, err = previewGroupConfiguration(ctx, []*agents.ManagedAgent{agent}, true, older, store)
	if err != nil {
		t.Fatal(err)
	}
	if eligible != 1 || preview[0].Reason != "Ready" {
		t.Fatalf("rollback preview=%+v eligible=%d", preview, eligible)
	}
}
