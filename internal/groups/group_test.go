// Tests controlled group identity, precedence, and disabled-group matching behavior.
package groups

import (
	"github.com/marellasunil/FleetAMP/internal/agents"
	"testing"
)

func TestMatchesManagedGroupIdentity(t *testing.T) {
	group := &Group{Enabled: true, Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{GroupFields: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if !Matches(group, agent) {
		t.Fatal("expected managed group identity to match")
	}
}

func TestMatchesReportedGroupIdentity(t *testing.T) {
	group := &Group{Enabled: true, Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{ReportedGroupFields: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if !Matches(group, agent) {
		t.Fatal("expected reported group identity to match")
	}
}

func TestLabelsDoNotDefineGroupMembership(t *testing.T) {
	group := &Group{Enabled: true, Selector: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	agent := &agents.ManagedAgent{Labels: map[string]string{"application": "payment-api", "environment": "prod", "place": "eu-west-1"}}
	if Matches(group, agent) {
		t.Fatal("labels must not define controlled group identity")
	}
}

func TestManagedGroupIdentityOverridesReported(t *testing.T) {
	group := &Group{Enabled: true, Selector: map[string]string{"environment": "dev"}}
	agent := &agents.ManagedAgent{ReportedGroupFields: map[string]string{"environment": "prod"}, GroupFields: map[string]string{"environment": "dev"}}
	if !Matches(group, agent) {
		t.Fatal("expected managed group identity to override reported value")
	}
}

func TestDisabledGroupDoesNotMatch(t *testing.T) {
	group := &Group{Enabled: false, Selector: map[string]string{"application": "payment-api"}}
	agent := &agents.ManagedAgent{GroupFields: map[string]string{"application": "payment-api"}}
	if Matches(group, agent) {
		t.Fatal("disabled group must not participate in active membership")
	}
	if !MatchesIdentity(group, agent) {
		t.Fatal("identity match must remain available for delete safety checks")
	}
}
