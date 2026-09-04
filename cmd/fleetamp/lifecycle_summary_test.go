package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/events"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type summaryEventStore struct {
	items []*events.AgentEvent
}

func (s summaryEventStore) ListSince(context.Context, time.Time) ([]*events.AgentEvent, error) {
	return s.items, nil
}

func TestPopulateLifecycleCountsIncludesRetiredOutsideTableFilter(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAgentStore()
	for _, agent := range []*agents.ManagedAgent{
		{InstanceUID: "active", Status: agents.LifecycleConnected},
		{InstanceUID: "offline", Status: agents.LifecycleDisconnected},
		{InstanceUID: "retired", Status: agents.LifecycleRetired},
	} {
		if err := store.Upsert(ctx, agent); err != nil {
			t.Fatal(err)
		}
	}

	view := agentListView{}
	populateLifecycleCounts(ctx, &view, store, nil)
	if view.Known != 3 || view.Active != 1 || view.Offline != 1 || view.Retired != 1 {
		t.Fatalf("unexpected lifecycle counts: known=%d active=%d offline=%d retired=%d", view.Known, view.Active, view.Offline, view.Retired)
	}
}

func TestPopulateLastConnectedShowsAgentAndGroupWithoutTimestamp(t *testing.T) {
	now := time.Now().UTC()
	group := &groups.Group{ID: "payments", Name: "Payments production", Enabled: true}
	view := agentListView{Items: []agentListItem{
		{Agent: &agents.ManagedAgent{InstanceUID: "older", Name: "collector-old"}},
		{Agent: &agents.ManagedAgent{InstanceUID: "newer", Name: "collector-new"}, Groups: []*groups.Group{group}},
	}}
	store := summaryEventStore{items: []*events.AgentEvent{
		{AgentInstanceUID: "older", Type: events.Connected, Timestamp: now.Add(-time.Minute)},
		{AgentInstanceUID: "newer", Type: events.Connected, Timestamp: now},
	}}

	populateLastConnected(context.Background(), &view, store)
	if view.LastConnectedAgent != "collector-new" {
		t.Fatalf("last connected agent=%q", view.LastConnectedAgent)
	}
	if view.LastConnectedGroup != "Payments production" {
		t.Fatalf("last connected group=%q", view.LastConnectedGroup)
	}
}

func TestManagedAgentsSectionRendersLifecycleSummaryWithoutLastSeenColumn(t *testing.T) {
	view := agentListView{
		Page: "fleet", Active: 3, Offline: 2, Retired: 1,
		LastConnectedAgent: "collector-new", LastConnectedGroup: "Payments production",
	}
	var output bytes.Buffer
	if err := agentsPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	html := output.String()
	for _, expected := range []string{"3 Active", "2 Offline", "1 Retired", "Last connected:", "Payments production"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("managed agents section missing %q", expected)
		}
	}
	if strings.Contains(html, "<th>Last seen</th>") {
		t.Fatal("managed agents table still contains Last seen timestamp column")
	}
	for _, removed := range []string{"Active / recent", ">1 hour<", ">24 hours<", ">7 days<"} {
		if strings.Contains(html, removed) {
			t.Fatalf("managed agents dropdown still contains time filter %q", removed)
		}
	}
	for _, status := range []string{">All agents<", ">Active<", ">Offline<", ">Retired<"} {
		if !strings.Contains(html, status) {
			t.Fatalf("managed agents dropdown missing lifecycle filter %q", status)
		}
	}
}

func TestSelectAgentsForStatus(t *testing.T) {
	ctx := context.Background()
	store := memory.NewAgentStore()
	for _, agent := range []*agents.ManagedAgent{
		{InstanceUID: "active", Status: agents.LifecycleConnected},
		{InstanceUID: "offline", Status: agents.LifecycleDisconnected},
		{InstanceUID: "retired", Status: agents.LifecycleRetired},
	} {
		if err := store.Upsert(ctx, agent); err != nil {
			t.Fatal(err)
		}
	}
	for status, expectedUID := range map[string]string{"active": "active", "offline": "offline", "retired": "retired"} {
		items, err := selectAgentsForStatus(ctx, store, status)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].InstanceUID != expectedUID {
			t.Fatalf("status %q returned %#v", status, items)
		}
	}
}
