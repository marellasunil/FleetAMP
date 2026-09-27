package main

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

func TestCanDeleteAgentUsesAssignedGroupOwnership(t *testing.T) {
	agent := &agents.ManagedAgent{GroupFields: map[string]string{
		"application": "payments", "environment": "prod", "place": "eu-west-1",
	}}
	owned := &groups.Group{Enabled: true, Owners: []string{"owner-one"}, Selector: map[string]string{
		"application": "payments", "environment": "prod", "place": "eu-west-1",
	}}

	if !canDeleteAgent(roleAdmin, "admin", agent, nil) {
		t.Fatal("admin must be allowed to delete an unassigned collector")
	}
	if !canDeleteAgent(roleGroupOwner, "owner-one", agent, []*groups.Group{owned}) {
		t.Fatal("assigned group owner must be allowed to delete the collector")
	}
	if canDeleteAgent(roleGroupOwner, "another-owner", agent, []*groups.Group{owned}) {
		t.Fatal("another group owner must not delete the collector")
	}
	if canDeleteAgent(roleGroupOwner, "owner-one", agent, nil) {
		t.Fatal("unassigned collectors must remain admin-only")
	}
}
