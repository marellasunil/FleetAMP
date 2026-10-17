package groups

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

func TestDynamicMatchesWildcardAndExclusion(t *testing.T) {
	group := &Group{
		ID:      "payments-prod",
		Enabled: true,
		MembershipRule: &MembershipRule{
			Field:     "stable_id",
			MatchType: MembershipMatchWildcard,
			Pattern:   "payments-prod-collector-*",
		},
	}
	agent := &agents.ManagedAgent{
		Name:       "otelcol",
		Attributes: map[string]string{"fleetamp.agent.id": "payments-prod-collector-07"},
	}
	if !DynamicMatches(group, agent) {
		t.Fatal("expected wildcard rule to match stable Collector ID")
	}
	agent.ExcludedGroupIDs = []string{group.ID}
	if DynamicMatches(group, agent) {
		t.Fatal("explicitly removed Collector must remain excluded")
	}
}

func TestDynamicMatchesReportedAttributeRegex(t *testing.T) {
	group := &Group{
		ID:      "payments",
		Enabled: true,
		MembershipRule: &MembershipRule{
			Field:     "attribute:k8s.deployment.name",
			MatchType: MembershipMatchRegex,
			Pattern:   "^payments-(api|worker)-[0-9]+$",
		},
	}
	matching := &agents.ManagedAgent{Attributes: map[string]string{"k8s.deployment.name": "payments-api-42"}}
	if !DynamicMatches(group, matching) {
		t.Fatal("expected reported attribute to match regex")
	}
	nonMatching := &agents.ManagedAgent{Attributes: map[string]string{"k8s.deployment.name": "orders-api-42"}}
	if DynamicMatches(group, nonMatching) {
		t.Fatal("unexpected match for unrelated deployment")
	}
}

func TestValidateMembershipRuleRejectsUnsafeValues(t *testing.T) {
	tests := []*MembershipRule{
		{Field: "unknown", MatchType: MembershipMatchWildcard, Pattern: "*"},
		{Field: "name", MatchType: "contains", Pattern: "payments"},
		{Field: "name", MatchType: MembershipMatchRegex, Pattern: "["},
	}
	for _, rule := range tests {
		if err := ValidateMembershipRule(rule); err == nil {
			t.Fatalf("expected validation error for %#v", rule)
		}
	}
}
