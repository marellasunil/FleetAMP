package main

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestConfigurationsForGroup(t *testing.T) {
	groupA := configs.NewGroupConfiguration("group-a", "a", "1", "service: {}", "text/yaml")
	groupB := configs.NewGroupConfiguration("group-b", "b", "1", "service: {}", "text/yaml")
	legacy := configs.NewConfiguration("legacy", "1", "service: {}", "text/yaml")

	got := configurationsForGroup([]*configs.Configuration{groupA, groupB, legacy, nil}, "group-a")
	if len(got) != 1 || got[0].ID != groupA.ID {
		t.Fatalf("unexpected group configurations: %#v", got)
	}
}
