package main

import (
	"strings"
	"testing"
)

func TestGroupDetailUsesOverflowManagementAndSummarySelector(t *testing.T) {
	for _, expected := range []string{
		`class="action-menu"`,
		`?edit=1#edit-group`,
		`id="edit-group"`,
		`href="/settings/users#group-owners"`,
		`href="/agents?group={{.Group.ID}}"`,
	} {
		if !strings.Contains(groupDetailHTML, expected) {
			t.Fatalf("group detail is missing %q", expected)
		}
	}
	for _, removed := range []string{
		`<div class="cardtitle">Assigned agents</div>`,
		`<div class="cardtitle">Group drift</div>`,
	} {
		if strings.Contains(groupDetailHTML, removed) {
			t.Fatalf("group detail still contains redundant section %q", removed)
		}
	}
}

func TestUserAdministrationOwnsGroupOwnerAssignment(t *testing.T) {
	for _, expected := range []string{
		`id="group-owners"`,
		`Set a user’s role to Group owner`,
		`FleetAMP keeps group ownership synchronized automatically`,
	} {
		if !strings.Contains(usersPageHTML, expected) {
			t.Fatalf("user administration is missing %q", expected)
		}
	}
}

func TestConfigurationTabsUseSegmentedNavigation(t *testing.T) {
	for _, expected := range []string{
		`.section-tabs{display:flex`,
		`overflow-x:auto`,
		`.section-tab.active:after`,
	} {
		if !strings.Contains(sectionEditorCSS, expected) {
			t.Fatalf("section tab styling is missing %q", expected)
		}
	}
}
