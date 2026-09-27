package configs

import "testing"

func TestGroupConfigurationIdentityIncludesGroup(t *testing.T) {
	first := NewGroupConfiguration("group-a", "collector.yaml", "1", "service: {}\n", "text/yaml")
	second := NewGroupConfiguration("group-b", "collector.yaml", "1", "service: {}\n", "text/yaml")
	if first.GroupID != "group-a" || second.GroupID != "group-b" {
		t.Fatalf("group IDs were not preserved: %q %q", first.GroupID, second.GroupID)
	}
	if first.ID == second.ID {
		t.Fatal("group-scoped configurations must have distinct identities")
	}
}

func TestUnscopedConfigurationIdentityRemainsCompatible(t *testing.T) {
	configuration := NewConfiguration("collector.yaml", "1", "service: {}\n", "text/yaml")
	if configuration.GroupID != "" {
		t.Fatalf("unexpected group ID %q", configuration.GroupID)
	}
	if configuration.ID != "3d2e81822097aeb7f8911b4c699635562246470bf3e985f47e21dbf6e0b0fa01" {
		t.Fatalf("legacy identity changed: %s", configuration.ID)
	}
}
