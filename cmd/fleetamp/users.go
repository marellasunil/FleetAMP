package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/groups"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

type userSummary struct {
	Username          string
	Email             string
	Role              string
	Enabled           bool
	Groups            []*groups.Group
	GroupOptions      []userGroupOption
	GroupIDs          []string
	Timezone          string
	LastLoginAt       *time.Time
	PasswordChangedAt time.Time
	AssignedRoles     []roleSummary
	RoleOptions       []userRoleOption
}

type userGroupOption struct {
	ID       string
	Name     string
	Selected bool
}

type userRoleOption struct {
	ID, Name string
	Selected bool
}

type accessGroupSummary struct {
	Group      *groups.Group
	Members    []userSummary
	Candidates []userSummary
}

type roleSummary struct {
	Name, ID, Scope, Description string
	Permissions                  []string
}

type usersPageData struct {
	Page         string
	Tab          string
	CurrentUser  string
	Users        []userSummary
	Owners       []userSummary
	Groups       []*groups.Group
	AccessGroups []accessGroupSummary
	Roles        []roleSummary
	Pagination   paginationView
	Message      string
	Error        string
}

const sessionJS = `(() => {
  fetch("/api/v1/session", {credentials:"same-origin", headers:{Accept:"application/json"}})
    .then((response) => response.ok ? response.json() : Promise.reject())
    .then((session) => {
      const identity = document.getElementById("current-user");
      if (identity) {
		const label = identity.querySelector("span:last-child");
		if (label) label.textContent = session.username + " · " + session.role;
      }
      document.querySelectorAll(".admin-settings-link").forEach((settings) => {
        if (session.role !== "admin") settings.hidden = true;
      });
      if (session.role === "group_owner") {
        document.querySelectorAll('nav a:not([href^="/groups"])').forEach((link) => { link.hidden = true; });
      }
    })
    .catch(() => {});
})();`

const usersPageHTML = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Users · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.userforms{display:grid;gap:8px}.useractions{display:flex;gap:8px;flex-wrap:wrap;align-items:end}
.useractions label{display:grid;gap:5px}.compact{min-width:125px}
.owner-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:10px}.owner-card{display:flex;justify-content:space-between;align-items:flex-start;gap:14px;padding:13px;border:1px solid #253a55;border-radius:9px;background:#0b1727}.owner-card .chips{justify-content:flex-end}
.membership-grid{display:grid;gap:7px;max-height:220px;min-width:220px;overflow:auto;padding:9px;border:1px solid #2a3d57;border-radius:8px;background:#0a1524}.membership-option{display:flex;gap:8px;align-items:center;color:var(--text)}.membership-actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:8px}.modal{max-width:520px;width:calc(100% - 32px);padding:22px;border:1px solid var(--line);border-radius:12px;background:var(--panel);color:var(--text)}.modal::backdrop{background:#020712cc}.chip button{border:0;background:transparent;color:inherit;cursor:pointer;font-weight:800}.action-row{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.action-row .btn,.action-row summary.btn{min-height:38px;height:38px;display:inline-flex;align-items:center;justify-content:center;padding:0 13px;line-height:1;white-space:nowrap}.action-row form{margin:0}.filter-dropdown{position:relative;min-width:230px}.filter-dropdown>summary{list-style:none;cursor:pointer;display:flex;align-items:center;width:100%;padding-right:34px;position:relative}.filter-dropdown>summary::-webkit-details-marker{display:none}.filter-dropdown>summary::after{content:"";position:absolute;right:14px;top:50%;width:7px;height:7px;border-right:2px solid var(--muted);border-bottom:2px solid var(--muted);transform:translateY(-70%) rotate(45deg)}.filter-dropdown[open]>summary::after{transform:translateY(-30%) rotate(225deg)}.filter-menu{position:absolute;z-index:30;top:calc(100% + 6px);left:0;width:min(360px,90vw);padding:12px;border:1px solid var(--line);border-radius:10px;background:var(--panel);box-shadow:0 16px 36px #020712cc}.filter-menu .chips{margin:8px 0}.filter-menu .membership-option{display:grid!important;grid-template-columns:18px minmax(0,1fr);align-items:center;gap:9px;text-align:left}.filter-menu .membership-option input{margin:0}.member-editor{position:relative}.member-editor>summary{list-style:none}.member-editor>summary::-webkit-details-marker{display:none}.member-panel{position:fixed;z-index:100;left:50%;right:auto;top:84px;transform:translateX(-50%);width:min(520px,calc(100vw - 32px));max-height:calc(100vh - 110px);overflow:auto;padding:16px;border:1px solid var(--line);border-radius:10px;background:var(--panel);box-shadow:0 20px 60px #020712ee}
</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Settings / Users</div>
<div class="pagetitle">Users, groups and roles</div>
<div class="subtitle">Manage users, group membership, roles, access, sign-in activity and credentials.</div></div>
<div class="topactions"><div class="connection"><span class="dot"></span>RBAC enforced</div></div></header>
<div class="content">
{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}
{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
<nav class="tabs" aria-label="Identity administration"><a class="tab {{if eq .Tab "users"}}active{{end}}" href="/settings/users?tab=users">Users</a><a class="tab {{if eq .Tab "group-members"}}active{{end}}" href="/settings/users?tab=group-members">Group Members</a><a class="tab {{if eq .Tab "roles"}}active{{end}}" href="/settings/users?tab=roles">Roles</a></nav>
{{if eq .Tab "users"}}<section class="card" style="margin-bottom:16px"><div class="cardhead"><div>
<div class="cardtitle">Create user</div>
<div class="cardsub">Passwords are protected by the server-specific FleetAMP pepper</div>
</div></div><div class="cardbody">
<form class="detailform" method="post" action="/settings/users">
<input type="hidden" name="action" value="create">
<label>Username<input class="input" name="username" required minlength="3" maxlength="64"></label>
<label>Email<input class="input" type="email" name="email" placeholder="owner@example.com"></label>
<label>Access type<select class="select" name="access" required><option value="member">Group member</option><option value="admin">Platform Admin</option></select></label>
<details class="filter-dropdown" data-multiselect><summary class="select">Select groups</summary><div class="filter-menu"><label>Search groups<input class="input" type="search" placeholder="Search group names" data-filter-options></label><div class="chips" data-selected-chips></div><div class="membership-grid">{{range .Groups}}<label class="membership-option" data-option-label="{{.Name}}"><input type="checkbox" name="group_ids" value="{{.ID}}" data-chip-label="{{.Name}}"> <span>{{.Name}}</span></label>{{else}}<span class="tiny">Create a group before adding a scoped user.</span>{{end}}</div><span class="tiny">Select one or more groups. Selected groups appear above.</span></div></details>
<details class="filter-dropdown" data-multiselect><summary class="select">Select roles</summary><div class="filter-menu"><label>Search roles<input class="input" type="search" placeholder="Search group roles" data-filter-options></label><div class="chips" data-selected-chips></div><div class="membership-grid">{{range .Roles}}{{if eq .Scope "Group"}}<label class="membership-option" data-option-label="{{.Name}}"><input type="checkbox" name="group_role_ids" value="{{.ID}}" data-chip-label="{{.Name}}"> <span>{{.Name}}</span></label>{{end}}{{end}}</div><span class="tiny">Roles apply within the selected groups only.</span></div></details>
<label>Password<input class="input" type="password" name="password" required minlength="16" autocomplete="new-password"></label>
<label>Confirm password<input class="input" type="password" name="confirm_password" required minlength="16" autocomplete="new-password"></label>
<button class="btn primary" type="submit">Create user</button>
</form></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Managed users</div>
<div class="cardsub">{{len .Users}} local FleetAMP user(s)</div></div></div>
{{if .Users}}<div style="overflow:auto"><table><thead><tr>
<th>User</th><th>Assigned roles</th><th>Group access</th><th>Last login</th><th>Status</th><th>Actions</th>
</tr></thead><tbody>{{range .Users}}<tr><td><strong>{{.Username}}</strong>
{{if .Email}}<div class="tiny">{{.Email}}</div>{{end}}{{if eq .Username $.CurrentUser}}<div class="tiny">Current session</div>{{end}}</td><td>
<div class="chips">{{range .AssignedRoles}}<span class="chip">{{.Name}}</span>{{else}}<span class="tiny">No assigned roles</span>{{end}}</div></td><td><div class="chips">{{range .Groups}}<a class="chip" href="/groups/{{.ID}}">{{.Name}}</a>{{else}}{{if eq .Role "admin"}}<span class="chip">All groups</span>{{else}}<span class="tiny">No group access</span>{{end}}{{end}}</div></td>
<td>{{if .LastLoginAt}}{{.LastLoginAt}}{{else}}<span class="tiny">Never</span>{{end}}<div class="tiny">Timezone: {{.Timezone}}</div></td><td><form method="post" action="/settings/users"><input type="hidden" name="action" value="status"><input type="hidden" name="username" value="{{.Username}}"><input type="hidden" name="enabled" value="{{if .Enabled}}false{{else}}true{{end}}"><button class="btn {{if .Enabled}}ok{{else}}off{{end}}" type="submit" title="Click to {{if .Enabled}}disable{{else}}enable{{end}} this user">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</button></form></td><td><div class="action-row"><a class="btn" href="/settings/users?tab=group-members&user={{.Username}}">Edit access</a><button class="btn" type="button" onclick="document.getElementById('reset-{{.Username}}').showModal()">Reset password</button></div><dialog id="reset-{{.Username}}" class="modal"><form class="userforms" method="post" action="/settings/users"><input type="hidden" name="action" value="reset_password"><input type="hidden" name="username" value="{{.Username}}"><strong>Reset password for {{.Username}}</strong><label>New password<input class="input" type="password" name="password" required minlength="16" autocomplete="new-password"></label><label>Confirm password<input class="input" type="password" name="confirm_password" required minlength="16" autocomplete="new-password"></label><div class="useractions"><button class="btn primary" type="submit">Reset</button><button class="btn" type="button" onclick="this.closest('dialog').close()">Cancel</button></div></form></dialog></td></tr>{{end}}
</tbody></table></div>{{else}}<div class="empty">No users configured.</div>{{end}}` + paginationHTML + `
</section>{{else if eq .Tab "group-members"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Group members</div><div class="cardsub">Manage people and their group-scoped roles without granting organization-wide access</div></div></div>{{if .AccessGroups}}<div style="overflow:auto"><table><thead><tr><th>Access group</th><th>Members</th><th>Collector scope</th><th>Status</th><th>Action</th></tr></thead><tbody>{{range .AccessGroups}}{{$group := .Group}}<tr><td><strong>{{.Group.Name}}</strong><div class="tiny code">{{.Group.ID}}</div></td><td>{{len .Members}}<div class="userforms">{{range .Members}}<div class="owner-card"><div><strong>{{.Username}}</strong><div class="chips">{{range .AssignedRoles}}<span class="chip">{{.Name}}</span>{{end}}</div></div><details class="member-editor"><summary class="btn">Edit</summary><div class="member-panel"><form class="userforms" method="post" action="/settings/users"><input type="hidden" name="action" value="membership"><input type="hidden" name="username" value="{{.Username}}"><input type="hidden" name="group_id" value="{{$group.ID}}"><strong>{{.Username}} · {{$group.Name}}</strong><div class="membership-grid">{{range .RoleOptions}}<label class="membership-option"><input type="checkbox" name="group_role_ids" value="{{.ID}}" {{if .Selected}}checked{{end}}> {{.Name}}</label>{{end}}</div><div class="action-row"><button class="btn primary" type="submit">Save roles</button><button class="btn" type="submit" name="remove" value="true">Remove member</button><button class="btn" type="button" onclick="this.closest('details').removeAttribute('open')">Cancel</button></div></form></div></details></div>{{else}}<span class="tiny">No users assigned</span>{{end}}</div></td><td>{{range $k,$v := .Group.Selector}}<span class="code">{{$k}}={{$v}}</span> {{end}}</td><td>{{if .Group.Enabled}}<span class="badge ok">Active</span>{{else}}<span class="badge off">Disabled</span>{{end}}</td><td><div class="action-row"><details class="member-editor"><summary class="btn primary">Add member</summary><div class="member-panel"><form class="userforms" method="post" action="/settings/users"><input type="hidden" name="action" value="membership"><input type="hidden" name="group_id" value="{{$group.ID}}"><strong>Add member to {{$group.Name}}</strong><label>Search user<input class="input" name="username" list="candidates-{{$group.ID}}" required placeholder="Type a username"></label><datalist id="candidates-{{$group.ID}}">{{range .Candidates}}<option value="{{.Username}}">{{.Email}}</option>{{end}}</datalist><div class="membership-grid">{{range $.Roles}}{{if eq .Scope "Group"}}<label class="membership-option"><input type="checkbox" name="group_role_ids" value="{{.ID}}"> {{.Name}}</label>{{end}}{{end}}</div><div class="action-row"><button class="btn primary" type="submit">Add member</button><button class="btn" type="button" onclick="this.closest('details').removeAttribute('open')">Cancel</button></div></form></div></details><a class="btn" href="/groups/{{$group.ID}}">View group</a></div></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No access scopes exist. Create a Collector Group first.</div>{{end}}</section>{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Built-in roles</div><div class="cardsub">Stable permission sets today; fine-grained custom permissions are planned as an additive migration</div></div></div><div class="cardbody groupgrid">{{range .Roles}}<div class="owner-card"><div><strong>{{.Name}}</strong><div class="tiny code">{{.ID}} · {{.Scope}}</div><p class="tiny">{{.Description}}</p><div class="chips">{{range .Permissions}}<span class="chip">{{.}}</span>{{end}}</div></div></div>{{end}}</div></section>{{end}}</div></main></div><script>
document.addEventListener('click', (event) => {
  document.querySelectorAll('details[open]').forEach((details) => {
    if (!details.contains(event.target)) details.removeAttribute('open');
  });
});
document.querySelectorAll('details').forEach((details) => {
  details.addEventListener('toggle', () => {
    if (!details.open) return;
    document.querySelectorAll('details[open]').forEach((other) => {
      if (other !== details && !other.contains(details)) other.removeAttribute('open');
    });
  });
});
document.querySelectorAll('[data-multiselect]').forEach((root) => {
  const search = root.querySelector('[data-filter-options]');
  const chips = root.querySelector('[data-selected-chips]');
  const options = [...root.querySelectorAll('[data-option-label]')];
  const render = () => {
    chips.replaceChildren();
    options.forEach((option) => {
      const checkbox = option.querySelector('input[type=checkbox]');
      if (!checkbox.checked) return;
      const chip = document.createElement('span');
      chip.className = 'chip';
      chip.append(document.createTextNode(checkbox.dataset.chipLabel + ' '));
      const remove = document.createElement('button');
      remove.type = 'button'; remove.textContent = '×';
      remove.setAttribute('aria-label', 'Remove ' + checkbox.dataset.chipLabel);
      remove.onclick = () => { checkbox.checked = false; render(); };
      chip.append(remove); chips.append(chip);
    });
  };
  search.addEventListener('input', () => {
    const value = search.value.toLowerCase();
    options.forEach((option) => { option.hidden = !option.dataset.optionLabel.toLowerCase().includes(value); });
  });
  options.forEach((option) => option.querySelector('input').addEventListener('change', render));
  render();
});
</script></body></html>`

var usersPage = template.Must(template.New("users").Parse(usersPageHTML))

type accountPageData struct {
	Page, Username, Email, Role, Timezone, Message, Error string
	Groups                                                []*groups.Group
	LastLoginAt                                           *time.Time
}

const accountPageHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>My account · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / My account</div><div class="pagetitle">{{.Username}}</div><div class="subtitle">Review your access and manage personal security and display preferences.</div></div></header><div class="content">{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}{{if .Error}}<div class="configerror">{{.Error}}</div>{{end}}<div class="detailgrid"><section class="card"><div class="cardhead"><div><div class="cardtitle">Access</div><div class="cardsub">Your effective FleetAMP scope</div></div></div><div class="cardbody"><div class="kv"><span>Role</span><strong>{{.Role}}</strong><span>Email</span><span>{{if .Email}}{{.Email}}{{else}}Not configured{{end}}</span><span>Last login</span><span>{{if .LastLoginAt}}{{.LastLoginAt}}{{else}}Current session is your first recorded login{{end}}</span><span>Groups</span><span>{{range .Groups}}<a class="chip" href="/groups/{{.ID}}">{{.Name}}</a> {{else}}{{if eq .Role "admin"}}All groups (Admin){{else}}No groups assigned{{end}}{{end}}</span></div></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Preferences</div><div class="cardsub">Used when FleetAMP displays local dates and times</div></div></div><div class="cardbody"><form class="detailform" method="post" action="/account"><input type="hidden" name="action" value="timezone"><label>Timezone<input class="input" name="timezone" value="{{.Timezone}}" placeholder="Europe/Amsterdam" required></label><button class="btn primary" type="submit">Save timezone</button></form></div></section><section class="card wide"><div class="cardhead"><div><div class="cardtitle">Reset password</div><div class="cardsub">Changing your password signs out all active sessions</div></div></div><div class="cardbody"><form class="detailform" method="post" action="/account"><input type="hidden" name="action" value="password"><label>New password<input class="input" type="password" name="password" required minlength="16" autocomplete="new-password"></label><label>Confirm password<input class="input" type="password" name="confirm_password" required minlength="16" autocomplete="new-password"></label><button class="btn primary" type="submit">Reset my password</button></form></div></section></div></div></main></div></body></html>`

var accountPage = template.Must(template.New("account").Parse(accountPageHTML))

func validRole(value string) bool {
	switch role(value) {
	case roleAdmin, roleOperator, roleGroupOwner, roleViewer:
		return true
	default:
		return false
	}
}

func validatedGroupRoleIDs(values []string) ([]string, error) {
	allowed := map[string]bool{}
	for _, item := range sqlitestore.BuiltinRBACRoles {
		allowed[item.ID] = true
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		if !allowed[value] {
			return nil, fmt.Errorf("unknown group role %q", value)
		}
		seen[value] = true
		result = append(result, value)
	}
	return result, nil
}

func compatibleLegacyRole(roleIDs []string) string {
	for _, roleID := range roleIDs {
		if roleID == "group_owner" || roleID == "deployment_approver" {
			return string(roleGroupOwner)
		}
	}
	for _, roleID := range roleIDs {
		if roleID == "configuration_editor" || roleID == "deployment_operator" {
			return string(roleOperator)
		}
	}
	return string(roleViewer)
}

func (a *authManager) updateGroupMembership(ctx context.Context, username, groupID string, roleIDs []string, remove bool) error {
	if _, err := a.groupStore.Get(ctx, groupID); err != nil {
		return fmt.Errorf("group %q does not exist", groupID)
	}
	roleIDs, err := validatedGroupRoleIDs(roleIDs)
	if err != nil {
		return err
	}
	if !remove && len(roleIDs) == 0 {
		return fmt.Errorf("select at least one group role")
	}
	all, err := a.store.ListMemberships(ctx)
	if err != nil {
		return err
	}
	next := make([]sqlitestore.GroupMembership, 0)
	allRoleIDs := make([]string, 0)
	groupIDs := make([]string, 0)
	replaced := false
	for _, membership := range all {
		if !strings.EqualFold(membership.Username, username) {
			continue
		}
		if membership.GroupID == groupID {
			replaced = true
			if remove {
				continue
			}
			membership.RoleIDs = roleIDs
		}
		next = append(next, membership)
		groupIDs = append(groupIDs, membership.GroupID)
		allRoleIDs = append(allRoleIDs, membership.RoleIDs...)
	}
	if !replaced && !remove {
		next = append(next, sqlitestore.GroupMembership{Username: username, GroupID: groupID, RoleIDs: roleIDs})
		groupIDs = append(groupIDs, groupID)
		allRoleIDs = append(allRoleIDs, roleIDs...)
	}
	user, err := a.store.Get(ctx, username)
	if err != nil {
		return err
	}
	if user.Role != string(roleAdmin) && len(next) == 0 {
		return fmt.Errorf("a non-Admin user must belong to at least one group")
	}
	if err := a.store.ReplaceMemberships(ctx, username, next); err != nil {
		return err
	}
	legacyRole := user.Role
	if user.Role != string(roleAdmin) {
		legacyRole = compatibleLegacyRole(allRoleIDs)
		if legacyRole != user.Role {
			if err := a.store.UpdateRole(ctx, username, legacyRole); err != nil {
				return err
			}
		}
	}
	if err := a.syncGroupOwnership(ctx, username, legacyRole, groupIDs); err != nil {
		return err
	}
	a.revokeUserSessions(username)
	return nil
}

func (a *authManager) createUser(ctx context.Context, username, email, password, access string, groupIDs, groupRoleIDs []string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	access = strings.ToLower(strings.TrimSpace(access))
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must contain between 3 and 64 characters")
	}
	if access != "member" && access != string(roleAdmin) {
		return fmt.Errorf("access must be group member or Platform Admin")
	}
	if email != "" {
		address, err := mail.ParseAddress(email)
		if err != nil || !strings.EqualFold(address.Address, email) {
			return fmt.Errorf("email address is invalid")
		}
	}
	if len(password) < minimumAdminPassword {
		return fmt.Errorf("password must contain at least %d characters", minimumAdminPassword)
	}
	groupIDs, err := a.validatedGroupIDs(ctx, groupIDs)
	if err != nil {
		return err
	}
	groupRoleIDs, err = validatedGroupRoleIDs(groupRoleIDs)
	if err != nil {
		return err
	}
	if access != string(roleAdmin) && (len(groupIDs) == 0 || len(groupRoleIDs) == 0) {
		return fmt.Errorf("select at least one group and one group role")
	}
	legacyRole := string(roleAdmin)
	if access != string(roleAdmin) {
		legacyRole = compatibleLegacyRole(groupRoleIDs)
	}
	salt, err := newSalt()
	if err != nil {
		return err
	}
	if err := a.store.Create(ctx, sqlitestore.User{
		Username: username, Email: email, Role: legacyRole, Enabled: true, GroupIDs: groupIDs, Timezone: "UTC",
		PasswordSalt: salt, PasswordHash: passwordDigest(password, a.pepper, salt),
	}); err != nil {
		return err
	}
	memberships := make([]sqlitestore.GroupMembership, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		memberships = append(memberships, sqlitestore.GroupMembership{Username: username, GroupID: groupID, RoleIDs: groupRoleIDs})
	}
	if err := a.store.ReplaceMemberships(ctx, username, memberships); err != nil {
		return err
	}
	return a.syncGroupOwnership(ctx, username, legacyRole, groupIDs)
}

func (a *authManager) validatedGroupIDs(ctx context.Context, values []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		if a.groupStore == nil {
			return nil, fmt.Errorf("group store is unavailable")
		}
		if _, err := a.groupStore.Get(ctx, id); err != nil {
			return nil, fmt.Errorf("group %q does not exist", id)
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func (a *authManager) syncGroupOwnership(ctx context.Context, username, userRole string, groupIDs []string) error {
	if a.groupStore == nil {
		return nil
	}
	selected := map[string]bool{}
	sawMembership := false
	if memberships, err := a.store.ListMemberships(ctx); err == nil {
		for _, membership := range memberships {
			if !strings.EqualFold(membership.Username, username) {
				continue
			}
			sawMembership = true
			for _, roleID := range membership.RoleIDs {
				if roleID == "group_owner" {
					selected[membership.GroupID] = true
					break
				}
			}
		}
	}
	if !sawMembership && userRole == string(roleGroupOwner) {
		for _, id := range groupIDs {
			selected[id] = true
		}
	}
	allGroups, err := a.groupStore.List(ctx)
	if err != nil {
		return err
	}
	for _, group := range allGroups {
		next := make([]string, 0, len(group.Owners)+1)
		for _, owner := range group.Owners {
			if !strings.EqualFold(owner, username) {
				next = append(next, owner)
			}
		}
		if selected[group.ID] {
			next = append(next, username)
		}
		next = normalizeOwners(next)
		if strings.Join(next, "\x00") == strings.Join(group.Owners, "\x00") {
			continue
		}
		group.Owners = next
		group.UpdatedAt = time.Now().UTC()
		if err := a.groupStore.Update(ctx, group); err != nil {
			return err
		}
	}
	return nil
}

func (a *authManager) revokeUserSessions(username string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, session := range a.sessions {
		if strings.EqualFold(session.Username, username) {
			delete(a.sessions, key)
		}
	}
}

func (a *authManager) resetUserPassword(ctx context.Context, username, password string) error {
	if len(password) < minimumAdminPassword {
		return fmt.Errorf("password must contain at least %d characters", minimumAdminPassword)
	}
	salt, err := newSalt()
	if err != nil {
		return err
	}
	if err := a.store.ReplacePassword(ctx, username, salt,
		passwordDigest(password, a.pepper, salt)); err != nil {
		return err
	}
	a.revokeUserSessions(username)
	return nil
}

func (a *authManager) registerUserRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/settings/users", a.handleUsers)
	mux.HandleFunc("/account", a.handleAccount)
	mux.HandleFunc("GET /api/v1/session", a.handleCurrentSession)
	mux.HandleFunc("GET /assets/session.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(sessionJS))
	})
}
func (a *authManager) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	username, ok := a.sessionUsername(r)
	if !ok {
		if legacyUsername, _, basicOK := r.BasicAuth(); basicOK {
			writeJSON(w, http.StatusOK, map[string]string{"username": legacyUsername, "role": string(roleAdmin)})
			return
		}
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	principalRole, ok := a.sessionRole(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	user, _ := a.store.Get(r.Context(), username)
	response := map[string]any{"username": username, "role": string(principalRole)}
	if user != nil {
		response["group_ids"] = user.GroupIDs
		response["timezone"] = user.Timezone
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *authManager) handleAccount(w http.ResponseWriter, r *http.Request) {
	username, ok := a.sessionUsername(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/account?error=Invalid+request", http.StatusSeeOther)
			return
		}
		var err error
		switch r.FormValue("action") {
		case "timezone":
			zone := strings.TrimSpace(r.FormValue("timezone"))
			if _, loadErr := time.LoadLocation(zone); loadErr != nil {
				err = fmt.Errorf("timezone must be a valid IANA name such as Europe/Amsterdam")
			} else {
				err = a.store.UpdateTimezone(r.Context(), username, zone)
			}
		case "password":
			if r.FormValue("password") != r.FormValue("confirm_password") {
				err = fmt.Errorf("passwords do not match")
			} else {
				err = a.resetUserPassword(r.Context(), username, r.FormValue("password"))
			}
		default:
			err = fmt.Errorf("unsupported account action")
		}
		values := url.Values{}
		if err != nil {
			values.Set("error", err.Error())
		} else {
			values.Set("message", "Account updated.")
		}
		http.Redirect(w, r, "/account?"+values.Encode(), http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, err := a.store.Get(r.Context(), username)
	if err != nil {
		internalServerError(w, err)
		return
	}
	view := accountPageData{Page: "account", Username: user.Username, Email: user.Email, Role: user.Role, Timezone: user.Timezone, LastLoginAt: user.LastLoginAt, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error")}
	for _, id := range user.GroupIDs {
		if group, groupErr := a.groupStore.Get(r.Context(), id); groupErr == nil {
			view.Groups = append(view.Groups, group)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = accountPage.Execute(w, view)
}

func (a *authManager) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/settings/users" {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		a.renderUsers(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		a.redirectUsers(w, r, "", "Invalid user-management request.")
		return
	}
	actor, _ := a.sessionUsername(r)
	target := strings.TrimSpace(r.FormValue("username"))
	action := strings.TrimSpace(r.FormValue("action"))
	var err error
	switch action {
	case "create":
		if r.FormValue("password") != r.FormValue("confirm_password") {
			err = fmt.Errorf("passwords do not match")
			break
		}
		err = a.createUser(r.Context(), target, r.FormValue("email"), r.FormValue("password"), r.FormValue("access"), r.Form["group_ids"], r.Form["group_role_ids"])
	case "role":
		nextRole := strings.ToLower(strings.TrimSpace(r.FormValue("role")))
		if !validRole(nextRole) {
			err = fmt.Errorf("invalid role")
			break
		}
		if strings.EqualFold(actor, target) && nextRole != string(roleAdmin) {
			err = fmt.Errorf("you cannot remove your own Admin role")
			break
		}
		existing, getErr := a.store.Get(r.Context(), target)
		if getErr != nil {
			err = getErr
			break
		}
		if existing.Role == nextRole {
			break
		}
		if nextRole != string(roleAdmin) && len(existing.GroupIDs) == 0 {
			err = fmt.Errorf("assign at least one group before changing this user to a scoped role")
			break
		}
		err = a.store.UpdateRole(r.Context(), target, nextRole)
		if err == nil {
			err = a.syncGroupOwnership(r.Context(), target, nextRole, existing.GroupIDs)
		}
		if err == nil {
			a.revokeUserSessions(target)
		}
	case "status":
		enabled := strings.EqualFold(r.FormValue("enabled"), "true")
		if strings.EqualFold(actor, target) && !enabled {
			err = fmt.Errorf("you cannot disable your current account")
			break
		}
		err = a.store.SetEnabled(r.Context(), target, enabled)
		if err == nil && !enabled {
			a.revokeUserSessions(target)
		}
	case "email":
		email := strings.TrimSpace(r.FormValue("email"))
		if email != "" {
			address, parseErr := mail.ParseAddress(email)
			if parseErr != nil || !strings.EqualFold(address.Address, email) {
				err = fmt.Errorf("email address is invalid")
				break
			}
		}
		err = a.store.UpdateEmail(r.Context(), target, email)
	case "groups":
		groupIDs, validationErr := a.validatedGroupIDs(r.Context(), r.Form["group_ids"])
		if validationErr != nil {
			err = validationErr
			break
		}
		existing, getErr := a.store.Get(r.Context(), target)
		if getErr != nil {
			err = getErr
			break
		}
		roleIDs, roleErr := validatedGroupRoleIDs(r.Form["group_role_ids"])
		if roleErr != nil {
			err = roleErr
			break
		}
		if existing.Role != string(roleAdmin) && (len(groupIDs) == 0 || len(roleIDs) == 0) {
			err = fmt.Errorf("select at least one group and one group role")
			break
		}
		memberships := make([]sqlitestore.GroupMembership, 0, len(groupIDs))
		for _, groupID := range groupIDs {
			memberships = append(memberships, sqlitestore.GroupMembership{Username: target, GroupID: groupID, RoleIDs: roleIDs})
		}
		err = a.store.ReplaceMemberships(r.Context(), target, memberships)
		nextLegacyRole := existing.Role
		if existing.Role != string(roleAdmin) {
			nextLegacyRole = compatibleLegacyRole(roleIDs)
			if err == nil && nextLegacyRole != existing.Role {
				err = a.store.UpdateRole(r.Context(), target, nextLegacyRole)
			}
		}
		if err == nil {
			err = a.syncGroupOwnership(r.Context(), target, nextLegacyRole, groupIDs)
		}
	case "membership":
		err = a.updateGroupMembership(r.Context(), target, strings.TrimSpace(r.FormValue("group_id")), r.Form["group_role_ids"], strings.EqualFold(r.FormValue("remove"), "true"))
	case "reset_password":
		if r.FormValue("password") != r.FormValue("confirm_password") {
			err = fmt.Errorf("passwords do not match")
			break
		}
		err = a.resetUserPassword(r.Context(), target, r.FormValue("password"))
	default:
		err = fmt.Errorf("unsupported user-management action")
	}
	if err != nil {
		if errors.Is(err, sqlitestore.ErrLastAdmin) {
			err = fmt.Errorf("at least one enabled Admin account is required")
		}
		slog.Warn("user management rejected", "component", "auth", "event", "user_management_rejected",
			"actor", actor, "target", target, "action", action, "error", err)
		a.redirectUsers(w, r, "", err.Error())
		return
	}
	slog.Info("user management completed", "component", "auth", "event", "user_management_completed",
		"actor", actor, "target", target, "action", action)
	message := map[string]string{
		"create": "User created.", "role": "Role updated.",
		"status": "User status updated.", "email": "Notification email updated.", "groups": "Group access updated.", "membership": "Group membership updated.", "reset_password": "Password reset.",
	}[action]
	a.redirectUsers(w, r, message, "")
}

func (a *authManager) redirectUsers(w http.ResponseWriter, r *http.Request, message, errorMessage string) {
	values := url.Values{}
	if message != "" {
		values.Set("message", message)
	}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	target := "/settings/users"
	if encoded := values.Encode(); encoded != "" {
		target += "?" + encoded
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func groupRolePermissions(roleID string) []string {
	switch roleID {
	case "group_owner":
		return []string{"Members", "Configurations", "Deployment requests"}
	case "configuration_editor":
		return []string{"Create versions", "Validate", "Submit for approval"}
	case "deployment_approver":
		return []string{"Approve", "Reject", "Send back"}
	case "deployment_operator":
		return []string{"Deploy approved", "Rollback", "Deployment status"}
	case "auditor":
		return []string{"Audit", "Drift", "Deployment history"}
	default:
		return []string{"Read group", "Read agents", "Read deployments"}
	}
}

func (a *authManager) renderUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	allGroups, err := a.groupStore.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	memberships, err := a.store.ListMemberships(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	storedRoles, err := a.store.ListRBACRoles(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	tab := r.URL.Query().Get("tab")
	switch tab {
	case "users", "group-members", "roles":
	default:
		tab = "users"
	}
	current, _ := a.sessionUsername(r)
	view := usersPageData{Page: "settings-users", Tab: tab, CurrentUser: current,
		Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error"),
		Users: make([]userSummary, 0, len(users)), Groups: allGroups}
	view.Roles = append(view.Roles, roleSummary{Name: "Platform Admin", ID: "admin", Scope: "Global",
		Description: "Full FleetAMP administration and access to every group.",
		Permissions: []string{"Identity and policy", "All groups", "All deployments", "Audit"}})
	roleByID := map[string]roleSummary{}
	for _, stored := range storedRoles {
		role := roleSummary{Name: stored.Name, ID: stored.ID, Scope: "Group", Description: stored.Description}
		role.Permissions = groupRolePermissions(stored.ID)
		view.Roles = append(view.Roles, role)
		roleByID[role.ID] = role
	}
	membershipByUser := map[string][]sqlitestore.GroupMembership{}
	membershipByGroup := map[string][]sqlitestore.GroupMembership{}
	for _, membership := range memberships {
		key := strings.ToLower(membership.Username)
		membershipByUser[key] = append(membershipByUser[key], membership)
		membershipByGroup[membership.GroupID] = append(membershipByGroup[membership.GroupID], membership)
	}
	userByName := map[string]userSummary{}
	for _, user := range users {
		summary := userSummary{Username: user.Username, Email: user.Email, Role: user.Role, Enabled: user.Enabled,
			GroupIDs: user.GroupIDs, Timezone: user.Timezone, LastLoginAt: user.LastLoginAt,
			PasswordChangedAt: user.PasswordChangedAt}
		selected := map[string]bool{}
		roleSeen := map[string]bool{}
		for _, membership := range membershipByUser[strings.ToLower(user.Username)] {
			selected[membership.GroupID] = true
			for _, roleID := range membership.RoleIDs {
				if role, ok := roleByID[roleID]; ok && !roleSeen[roleID] {
					summary.AssignedRoles = append(summary.AssignedRoles, role)
					roleSeen[roleID] = true
				}
			}
		}
		if user.Role == string(roleAdmin) {
			summary.AssignedRoles = []roleSummary{view.Roles[0]}
		}
		for _, group := range allGroups {
			summary.GroupOptions = append(summary.GroupOptions, userGroupOption{ID: group.ID, Name: group.Name, Selected: selected[group.ID]})
			if selected[group.ID] {
				summary.Groups = append(summary.Groups, group)
			}
		}
		view.Users = append(view.Users, summary)
		userByName[strings.ToLower(user.Username)] = summary
	}
	for _, group := range allGroups {
		item := accessGroupSummary{Group: group}
		for _, membership := range membershipByGroup[group.ID] {
			if member, ok := userByName[strings.ToLower(membership.Username)]; ok {
				member.AssignedRoles = nil
				selectedRoles := map[string]bool{}
				for _, roleID := range membership.RoleIDs {
					selectedRoles[roleID] = true
					if role, found := roleByID[roleID]; found {
						member.AssignedRoles = append(member.AssignedRoles, role)
					}
				}
				member.RoleOptions = nil
				for _, storedRole := range storedRoles {
					member.RoleOptions = append(member.RoleOptions, userRoleOption{
						ID: storedRole.ID, Name: storedRole.Name, Selected: selectedRoles[storedRole.ID],
					})
				}
				item.Members = append(item.Members, member)
			}
		}
		memberNames := map[string]bool{}
		for _, member := range item.Members {
			memberNames[strings.ToLower(member.Username)] = true
		}
		for _, candidate := range view.Users {
			if candidate.Role == string(roleAdmin) || memberNames[strings.ToLower(candidate.Username)] {
				continue
			}
			candidate.RoleOptions = nil
			for _, storedRole := range storedRoles {
				candidate.RoleOptions = append(candidate.RoleOptions, userRoleOption{ID: storedRole.ID, Name: storedRole.Name})
			}
			item.Candidates = append(item.Candidates, candidate)
		}
		view.AccessGroups = append(view.AccessGroups, item)
	}
	view.Pagination = paginationFromRequest(r, len(view.Users))
	view.Users = paginateSlice(view.Users, view.Pagination)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := usersPage.Execute(w, view); err != nil {
		slog.Error("render users page", "component", "http", "error", err)
	}
}
