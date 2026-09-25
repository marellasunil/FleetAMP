package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/audit"
)

func TestAuditStoreAppendsAndFilters(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := database.Audit()
	base := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	events := []*audit.Event{
		{Timestamp: base, Actor: "admin", Action: "configuration.deploy", ResourceType: "agent", Outcome: "success", HTTPMethod: "POST", Path: "/agents/a/config", StatusCode: 303},
		{Timestamp: base.Add(time.Minute), Actor: "operator", Action: "configuration.deploy", ResourceType: "agent", Outcome: "denied", HTTPMethod: "POST", Path: "/agents/a/config", StatusCode: 403},
		{Timestamp: base.Add(2 * time.Minute), Actor: "admin", Action: "policy.drift_update", ResourceType: "configuration_policy", Outcome: "success", HTTPMethod: "POST", Path: "/settings/configuration-drift", StatusCode: 303},
	}
	for _, event := range events {
		if err := store.Append(ctx, event); err != nil {
			t.Fatal(err)
		}
	}

	result, err := store.List(ctx, audit.Filter{Actor: "admin", Outcome: "success", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Fatalf("events=%d, want 2", len(result))
	}
	if result[0].Action != "policy.drift_update" {
		t.Fatalf("newest action=%q", result[0].Action)
	}
	result, err = store.List(ctx, audit.Filter{
		Since: base.Add(30 * time.Second), Until: base.Add(90 * time.Second), Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Actor != "operator" {
		t.Fatalf("date-filtered events=%#v", result)
	}
}
