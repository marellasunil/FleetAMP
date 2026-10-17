package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestMembershipPageExposesGovernedCollectorOperations(t *testing.T) {
	var body bytes.Buffer
	err := membershipPage.Execute(&body, membershipView{Group: &groups.Group{ID: "payments", Name: "Payments"}, CanManage: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"Assign available Collectors", "Remove from this group", "Move to another group", "Restore pre-group configuration", "Destination configuration", "Impact preview", "membership-preview.js", "Membership history"} {
		if !strings.Contains(body.String(), text) {
			t.Errorf("page missing %q", text)
		}
	}
}

func TestMembershipBaselineUsesMostRecentCompletedEntry(t *testing.T) {
	history := []*configs.MembershipChange{
		{Status: configs.MembershipFailed, Operation: configs.MembershipMove, Targets: []configs.MembershipTarget{{AgentInstanceUID: "agent-1", PreviousConfigurationID: "failed"}}},
		{Status: configs.MembershipCompleted, Operation: configs.MembershipMove, Targets: []configs.MembershipTarget{{AgentInstanceUID: "agent-1", PreviousConfigurationID: "source-config"}}},
	}
	if got := membershipBaseline("agent-1", history); got != "source-config" {
		t.Fatalf("baseline=%q", got)
	}
}

func TestRollbackMembershipBatchRestoresMembershipAndConfiguration(t *testing.T) {
	ctx := context.Background()
	agentStore := memory.NewAgentStore()
	configStore := memory.NewConfigStore()
	previous := configs.NewConfiguration("previous", "1", "receivers: {}", "yaml")
	if err := configStore.Put(ctx, previous); err != nil {
		t.Fatal(err)
	}
	original := &agents.ManagedAgent{InstanceUID: "agent-1", Name: "collector-1", GroupFields: map[string]string{"team": "source"}, ExcludedGroupIDs: []string{"old"}}
	changed := *original
	changed.GroupFields = map[string]string{"team": "target"}
	changed.ExcludedGroupIDs = nil
	if err := agentStore.Upsert(ctx, &changed); err != nil {
		t.Fatal(err)
	}
	var deliveredID string
	var deliveredAction configs.DeploymentAction
	deliver := func(_ context.Context, _ string, configuration *configs.Configuration, action configs.DeploymentAction, _, _ string) error {
		deliveredID, deliveredAction = configuration.ID, action
		return nil
	}
	cause := errors.New("second Collector failed")
	err := rollbackMembershipBatch(ctx, &configs.MembershipChange{ID: "request-1"}, []*membershipExecutionCheckpoint{{
		agent: changedAgent(t, agentStore, "agent-1"), original: original, previousConfigurationID: previous.ID, configurationChanged: true, membershipChanged: true,
	}}, agentStore, configStore, deliver, t.TempDir(), cause)
	if err == nil || !strings.Contains(err.Error(), "all previously changed Collectors were restored") {
		t.Fatalf("rollback error=%v", err)
	}
	restored, getErr := agentStore.Get(ctx, "agent-1")
	if getErr != nil {
		t.Fatal(getErr)
	}
	if restored.GroupFields["team"] != "source" || len(restored.ExcludedGroupIDs) != 1 || restored.ExcludedGroupIDs[0] != "old" {
		t.Fatalf("membership was not restored: %#v", restored)
	}
	if deliveredID != previous.ID || deliveredAction != configs.DeploymentActionRollback {
		t.Fatalf("rollback delivery=%q action=%q", deliveredID, deliveredAction)
	}
}

func changedAgent(t *testing.T, store *memory.AgentStore, uid string) *agents.ManagedAgent {
	t.Helper()
	agent, err := store.Get(context.Background(), uid)
	if err != nil {
		t.Fatal(err)
	}
	return agent
}
