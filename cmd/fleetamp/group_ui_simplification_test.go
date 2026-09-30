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

func TestUserAdministrationSeparatesRolesAndGroupMembership(t *testing.T) {
	for _, expected := range []string{
		`<th>Assigned roles</th><th>Group access</th>`,
		`name="group_ids"`,
		`name="group_role_ids"`,
		`data-selected-chips`,
		`Search groups`,
		`Group Members`,
	} {
		if !strings.Contains(usersPageHTML, expected) {
			t.Fatalf("user administration is missing %q", expected)
		}
	}
}

func TestUserAdministrationUsesWorkingDropdownsAndAlignedActions(t *testing.T) {
	for _, expected := range []string{
		`<details class="filter-dropdown" data-multiselect>`,
		`<summary class="select">Select groups</summary>`,
		`<summary class="select">Select roles</summary>`,
		`<details class="member-editor"><summary class="btn primary">Add member</summary>`,
		`<details class="member-editor"><summary class="btn">Edit</summary>`,
		`<input type="hidden" name="enabled" value="{{if .Enabled}}false{{else}}true{{end}}">`,
		`{{if .Enabled}}Enabled{{else}}Disabled{{end}}`,
		`.filter-menu .membership-option{display:grid!important;grid-template-columns:18px minmax(0,1fr)`,
		`.member-panel{position:fixed`,
		`document.addEventListener('click', (event) => {`,
		`.action-row .btn,.action-row summary.btn{min-height:38px;height:38px`,
	} {
		if !strings.Contains(usersPageHTML, expected) {
			t.Fatalf("user administration is missing %q", expected)
		}
	}
	for _, blocked := range []string{
		`id="add-{{$group.ID}}"`,
		`id="member-{{$group.ID}}-{{.Username}}"`,
		`<span class="badge {{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</span>`,
	} {
		if strings.Contains(usersPageHTML, blocked) {
			t.Fatalf("group membership still relies on dialog trigger %q", blocked)
		}
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

func TestGroupMembershipAdministrationSupportsScopedRoles(t *testing.T) {
	for _, expected := range []string{
		`Add member`,
		`Save roles`,
		`Remove member`,
		`Platform Admin`,
	} {
		if !strings.Contains(usersPageHTML, expected) {
			t.Fatalf("group membership administration is missing %q", expected)
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
