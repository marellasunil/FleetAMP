package main

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

func TestMigrationVisibleCollectorsEnforcesTypeLifecycleAndGroupScope(t *testing.T) {
	group := &groups.Group{ID: "payments", Enabled: true, Selector: map[string]string{"application": "payments"}}
	items := []*agents.ManagedAgent{
		{InstanceUID: "visible", Name: "A", Type: agents.AgentTypeOTelCollector, ReportedGroupFields: map[string]string{"application": "payments"}},
		{InstanceUID: "other-group", Name: "B", Type: agents.AgentTypeOTelCollector, ReportedGroupFields: map[string]string{"application": "orders"}},
		{InstanceUID: "alloy", Name: "C", Type: agents.AgentTypeGrafanaAlloy, ReportedGroupFields: map[string]string{"application": "payments"}},
		{InstanceUID: "retired", Name: "D", Type: agents.AgentTypeOTelCollector, Status: agents.LifecycleRetired, ReportedGroupFields: map[string]string{"application": "payments"}},
	}

	operatorView := migrationVisibleCollectors(items, []*groups.Group{group}, roleOperator)
	if len(operatorView) != 1 || operatorView[0].InstanceUID != "visible" {
		t.Fatalf("operator collectors=%v", operatorView)
	}
	adminView := migrationVisibleCollectors(items, []*groups.Group{group}, roleAdmin)
	if len(adminView) != 2 {
		t.Fatalf("admin collectors=%v", adminView)
	}
}

func TestAssessCollectorAdoptionReady(t *testing.T) {
	group := &groups.Group{ID: "payments", Enabled: true, Selector: map[string]string{"application": "payments"}}
	agent := &agents.ManagedAgent{
		InstanceUID: "agent-1",
		Type:        agents.AgentTypeOTelCollector,
		Connected:   true,
		Healthy:     true,
		Attributes:  map[string]string{stableAgentIDAttribute: "payment-api-prod-collector-node-1"},
		GroupFields: map[string]string{"application": "payments"},
		Capabilities: []string{
			"accepts_remote_config",
			"reports_effective_config",
		},
	}
	checks, ready := assessCollectorAdoption(agent, group, "receivers: {}\n")
	if !ready || len(checks) != 6 {
		t.Fatalf("ready=%t checks=%v", ready, checks)
	}
	for _, check := range checks {
		if check.Status != "Ready" || check.Blocking {
			t.Fatalf("unexpected readiness check: %+v", check)
		}
	}
}

func TestAssessCollectorAdoptionBlocksUnsafeOwnershipAndCapabilities(t *testing.T) {
	group := &groups.Group{ID: "payments", Enabled: true, Selector: map[string]string{"application": "payments"}}
	agent := &agents.ManagedAgent{
		InstanceUID:  "agent-1",
		Type:         agents.AgentTypeOTelCollector,
		Connected:    false,
		Healthy:      false,
		GroupFields:  map[string]string{"application": "orders"},
		Capabilities: []string{},
	}
	checks, ready := assessCollectorAdoption(agent, group, "")
	if ready {
		t.Fatal("collector with missing identity, config, group match and capabilities was ready")
	}
	blocking := 0
	for _, check := range checks {
		if check.Blocking {
			blocking++
		}
	}
	if blocking != 5 {
		t.Fatalf("blocking checks=%d, want 5: %v", blocking, checks)
	}
}

func TestMigrationCollectorByIDDoesNotGuess(t *testing.T) {
	items := []*agents.ManagedAgent{{InstanceUID: "one"}, {InstanceUID: "two"}}
	if got := migrationCollectorByID(items, "two"); got == nil || got.InstanceUID != "two" {
		t.Fatalf("collector=%v", got)
	}
	if got := migrationCollectorByID(items, "missing"); got != nil {
		t.Fatalf("unexpected collector=%v", got)
	}
}
