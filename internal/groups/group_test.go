package groups

import (
	"github.com/marellasunil/FleetAMP/internal/agents"
	"testing"
)

func TestMatchesManagedGroupIdentity(t *testing.T) {
	group := &Group{Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{GroupFields: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if !Matches(group, agent) {
		t.Fatal("expected managed group identity to match")
	}
}

func TestMatchesReportedGroupIdentity(t *testing.T) {
	group := &Group{Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{ReportedGroupFields: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if !Matches(group, agent) {
		t.Fatal("expected reported group identity to match")
	}
}

func TestLabelsDoNotDefineGroupMembership(t *testing.T) {
	group := &Group{Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{Labels: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if Matches(group, agent) {
		t.Fatal("labels must not define controlled group identity")
	}
}

func TestManagedGroupIdentityOverridesReported(t *testing.T) {
	group := &Group{Selector: map[string]string{"environment": "dev"}}
	agent := &agents.ManagedAgent{ReportedGroupFields: map[string]string{"environment": "prod"}, GroupFields: map[string]string{"environment": "dev"}}
	if !Matches(group, agent) {
		t.Fatal("expected managed group identity to override reported value")
	}
}
