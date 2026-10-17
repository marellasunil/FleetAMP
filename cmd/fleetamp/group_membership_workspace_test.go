package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func TestGroupWorkspaceTemplateHasDedicatedViews(t *testing.T) {
	for _, want := range []string{
		"All Groups",
		"Create Group",
		"Unassigned Collectors",
		"Dynamic fleet membership",
		"data-live-search=\"groups\"",
		"data-live-search=\"create-collectors\"",
	} {
		if !strings.Contains(groupsHTML, want) {
			t.Fatalf("groups workspace is missing %q", want)
		}
	}
}

func TestGroupMatchesQueryIncludesLabelsAndDynamicRule(t *testing.T) {
	group := &groups.Group{
		Name:        "Payments production",
		Description: "EU payment fleet",
		Selector:    map[string]string{"application": "payments", "environment": "prod"},
		MembershipRule: &groups.MembershipRule{
			Field:     "stable_id",
			MatchType: groups.MembershipMatchWildcard,
			Pattern:   "payments-prod-*",
		},
	}
	for _, query := range []string{"payments production", "environment=prod", "stable_id", "payments-prod-*"} {
		if !groupMatchesQuery(group, query) {
			t.Fatalf("expected query %q to match", query)
		}
	}
	if groupMatchesQuery(group, "orders") {
		t.Fatal("unexpected match for unrelated query")
	}
}

func TestUnassignedCollectorsReportsDynamicConflict(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAgentStore()
	agent := &agents.ManagedAgent{
		InstanceUID: "collector-1",
		Name:        "payments-prod-01",
		Attributes:  map[string]string{"host.name": "payments-prod-01"},
	}
	if err := store.Upsert(ctx, agent); err != nil {
		t.Fatal(err)
	}
	makeGroup := func(id, pattern string) *groups.Group {
		return &groups.Group{ID: id, Name: id, Enabled: true, MembershipRule: &groups.MembershipRule{
			Field: "host.name", MatchType: groups.MembershipMatchWildcard, Pattern: pattern,
		}}
	}
	items, err := buildUnassignedCollectorItems(ctx, store, []*groups.Group{
		makeGroup("group-a", "payments-*"),
		makeGroup("group-b", "*-prod-*"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Conflict, "Matches multiple groups") {
		t.Fatalf("expected one visible conflict, got %#v", items)
	}
}

func TestAssignCollectorsDuringGroupCreationAssignsOnlyUnownedCollectors(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	db, err := sqlitestore.Open(ctx, filepath.Join(dataDir, "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	groupStore := db.Groups()
	agentStore := memory.NewAgentStore()
	agent := &agents.ManagedAgent{InstanceUID: "collector-1", Name: "payments-prod-01"}
	if err := agentStore.Upsert(ctx, agent); err != nil {
		t.Fatal(err)
	}

	payments, err := newValidatedGroup(groupRequest{Name: "Payments", Selector: map[string]string{"application": "payments"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := groupStore.Create(ctx, payments); err != nil {
		t.Fatal(err)
	}
	if err := assignCollectorsDuringGroupCreation(ctx, payments, []string{agent.InstanceUID}, groupStore, agentStore, dataDir); err != nil {
		t.Fatal(err)
	}
	assigned, err := agentStore.Get(ctx, agent.InstanceUID)
	if err != nil {
		t.Fatal(err)
	}
	if !groups.Matches(payments, assigned) {
		t.Fatalf("Collector was not assigned to the new group: %+v", assigned.GroupFields)
	}

	orders, err := newValidatedGroup(groupRequest{Name: "Orders", Selector: map[string]string{"application": "orders"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := groupStore.Create(ctx, orders); err != nil {
		t.Fatal(err)
	}
	if err := assignCollectorsDuringGroupCreation(ctx, orders, []string{agent.InstanceUID}, groupStore, agentStore, dataDir); err == nil || !strings.Contains(err.Error(), "already belongs") {
		t.Fatalf("expected existing ownership to block reassignment, got %v", err)
	}
}
