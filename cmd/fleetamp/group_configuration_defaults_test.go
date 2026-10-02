package main

import (
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestPreferredGroupConfigurationUsesLatestAppliedDeployment(t *testing.T) {
	savedLatest := configs.NewGroupConfiguration("group-1", "collector.yaml", "3.0.0", "receivers: {}\n", "text/yaml")
	deployed := configs.NewGroupConfiguration("group-1", "collector.yaml", "2.0.0", "receivers:\n  otlp: {}\n", "text/yaml")
	history := []*configs.Deployment{
		{ConfigurationID: savedLatest.ID, Status: configs.DeliveryFailed, CreatedAt: time.Now().UTC()},
		{ConfigurationID: deployed.ID, Status: configs.DeliveryApplied, CreatedAt: time.Now().Add(-time.Minute).UTC()},
	}
	if got := preferredGroupConfiguration([]*configs.Configuration{savedLatest, deployed}, history, ""); got != deployed {
		t.Fatalf("selected %#v, want latest applied deployment %#v", got, deployed)
	}
}

func TestPreferredGroupConfigurationHonorsExplicitBaseline(t *testing.T) {
	latest := configs.NewGroupConfiguration("group-1", "collector.yaml", "2.0.0", "receivers: {}\n", "text/yaml")
	older := configs.NewGroupConfiguration("group-1", "collector.yaml", "1.0.0", "receivers:\n  otlp: {}\n", "text/yaml")
	if got := preferredGroupConfiguration([]*configs.Configuration{latest, older}, nil, older.ID); got != older {
		t.Fatalf("selected %#v, want explicitly requested baseline %#v", got, older)
	}
}

func TestGroupConfigurationUIUsesDeploymentBaselineAndBlueprint(t *testing.T) {
	for _, expected := range []string{
		`href="/blueprints?group_id={{.Group.ID}}"`,
		`value="{{.EditorName}}"`,
		`value="{{.EditorVersion}}"`,
		"latest successfully deployed version",
		"Unchanged content or a reused version is rejected",
	} {
		if !strings.Contains(groupDetailHTML, expected) {
			t.Fatalf("group configuration UI is missing %q", expected)
		}
	}
}
