package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

func TestGroupDetailUsesOverflowManagementWithoutOverviewCards(t *testing.T) {
	for _, expected := range []string{
		`class="action-menu"`,
		`?edit=1#edit-group`,
		`id="edit-group"`,
		`Deletion blocked: {{len .Members}} Collector(s) still belong to this group`,
		`including {{.ActiveMembers}} active`,
	} {
		if !strings.Contains(groupDetailHTML, expected) {
			t.Fatalf("group detail is missing %q", expected)
		}
	}
	for _, removed := range []string{
		`<div class="cardtitle">Group selector</div>`,
		`Group-level desired state, approvals and drift`,
		`<div class="cardtitle">Assigned agents</div>`,
		`<div class="cardtitle">Group drift</div>`,
	} {
		if strings.Contains(groupDetailHTML, removed) {
			t.Fatalf("group detail still contains redundant section %q", removed)
		}
	}
}

func TestGroupsPageProvidesSafeCardActions(t *testing.T) {
	for _, expected := range []string{
		`aria-label="Actions for {{.Group.Name}}"`,
		`/groups/{{.Group.ID}}?edit=1#edit-group`,
		`name="action" value="{{if .Group.Enabled}}disable{{else}}enable{{end}}"`,
		`Deletion blocked: {{.MemberCount}} Collector(s) still belong to this group`,
		`including {{.ActiveMemberCount}} active`,
		`Delete this empty ownership group?`,
		`Configuration state`,
		`{{.SavedVersions}}`,
		`{{.ApprovalRequests}}`,
	} {
		if !strings.Contains(groupsHTML, expected) {
			t.Fatalf("groups page is missing %q", expected)
		}
	}
}

func TestUserAdministrationSeparatesRoleAndMultiGroupMembership(t *testing.T) {
	for _, expected := range []string{
		`<th>Role</th><th>Groups</th>`,
		`type="checkbox" name="group_ids"`,
		`Save selected`,
		`Remove all`,
		`Check or uncheck several groups, then save once.`,
	} {
		if !strings.Contains(usersPageHTML, expected) {
			t.Fatalf("user administration is missing %q", expected)
		}
	}
	if strings.Contains(usersPageHTML, `Role and group access`) {
		t.Fatal("user administration still combines role and groups in one column")
	}
}

func TestGroupMoveAuditDetailsPreserveDeploymentContext(t *testing.T) {
	request := httptest.NewRequest("POST", "/agents/collector-1/group", strings.NewReader(url.Values{
		"audit_previous_group": {"Payments NL Dev"},
		"audit_new_group":      {"Payments NL Prod"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	details := deploymentAuditDetails(request)
	for _, expected := range []string{"Payments NL Dev → Payments NL Prod", "deployment history retained", "desired state"} {
		if !strings.Contains(details, expected) {
			t.Fatalf("group move audit details %q do not contain %q", details, expected)
		}
	}
}

func TestActiveGroupMemberCount(t *testing.T) {
	members := []*agents.ManagedAgent{
		{Connected: true, Status: agents.LifecycleConnected},
		{Connected: false, Status: agents.LifecycleDisconnected},
		{Connected: true, Status: agents.LifecycleRetired},
	}
	if got := activeGroupMemberCount(members); got != 1 {
		t.Fatalf("activeGroupMemberCount() = %d, want 1", got)
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
