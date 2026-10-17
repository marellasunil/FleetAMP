package main

import (
	"bytes"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"strings"
	"testing"
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
