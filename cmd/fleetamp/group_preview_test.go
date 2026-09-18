package main

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

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
