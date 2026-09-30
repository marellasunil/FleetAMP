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
}

type userGroupOption struct {
	ID       string
	Name     string
	Selected bool
}

type usersPageData struct {
	Page        string
	CurrentUser string
	Users       []userSummary
	Owners      []userSummary
	Groups      []*groups.Group
	Message     string
	Error       string
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
.membership-grid{display:grid;gap:7px;max-height:180px;min-width:220px;overflow:auto;padding:9px;border:1px solid #2a3d57;border-radius:8px;background:#0a1524}.membership-option{display:flex;gap:8px;align-items:center;color:var(--text)}.membership-actions{display:flex;gap:8px;flex-wrap:wrap;margin-top:8px}
</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Settings / Users</div>
<div class="pagetitle">Users, groups and roles</div>
<div class="subtitle">Manage users, group membership, roles, access, sign-in activity and credentials.</div></div>
<div class="topactions"><div class="connection"><span class="dot"></span>RBAC enforced</div></div></header>
<div class="content">
{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}
{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
<section class="card" style="margin-bottom:16px"><div class="cardhead"><div>
<div class="cardtitle">Create user</div>
<div class="cardsub">Passwords are protected by the server-specific FleetAMP pepper</div>
</div></div><div class="cardbody">
<form class="detailform" method="post" action="/settings/users">
<input type="hidden" name="action" value="create">
<label>Username<input class="input" name="username" required minlength="3" maxlength="64"></label>
<label>Email<input class="input" type="email" name="email" placeholder="owner@example.com"></label>
<label>Role<select class="select" name="role" required>
<option value="viewer">Viewer</option><option value="group_owner">Group owner</option><option value="operator">Operator</option>
<option value="admin">Admin</option></select></label>
<div><div style="margin-bottom:7px">Groups</div><div class="membership-grid">{{range .Groups}}<label class="membership-option"><input type="checkbox" name="group_ids" value="{{.ID}}"> <span>{{.Name}}</span></label>{{else}}<span class="tiny">Create a group before adding a scoped user.</span>{{end}}</div><span class="tiny">Select any number of groups for scoped access.</span></div>
<label>Password<input class="input" type="password" name="password" required minlength="16" autocomplete="new-password"></label>
<label>Confirm password<input class="input" type="password" name="confirm_password" required minlength="16" autocomplete="new-password"></label>
<button class="btn primary" type="submit">Create user</button>
</form></div></section>
<section class="card" id="group-owners" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Group owners</div><div class="cardsub">Ownership is assigned from each user’s role and group membership</div></div><span class="badge off">{{len .Owners}} owner(s)</span></div><div class="cardbody">{{if .Owners}}<div class="owner-grid">{{range .Owners}}<div class="owner-card"><div><strong>{{.Username}}</strong>{{if .Email}}<div class="tiny">{{.Email}}</div>{{end}}</div><div class="chips">{{range .Groups}}<a class="chip" href="/groups/{{.ID}}">{{.Name}}</a>{{else}}<span class="tiny">No group assigned</span>{{end}}</div></div>{{end}}</div>{{else}}<div class="empty">No Group Owners configured. Set a user’s role to Group owner and assign one or more groups below.</div>{{end}}<p class="tiny" style="margin-top:14px">To add or remove an owner, update that user’s role and group access in Managed users. FleetAMP keeps group ownership synchronized automatically.</p></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Managed users</div>
<div class="cardsub">{{len .Users}} local FleetAMP user(s)</div></div></div>
{{if .Users}}<div style="overflow:auto"><table><thead><tr>
<th>User and email</th><th>Role</th><th>Groups</th><th>Last login</th><th>Status</th><th>Password</th>
</tr></thead><tbody>{{range .Users}}<tr><td><strong>{{.Username}}</strong>
{{if eq .Username $.CurrentUser}}<div class="tiny">Current session</div>{{end}}
<form class="useractions" method="post" action="/settings/users" style="margin-top:8px"><input type="hidden" name="action" value="email"><input type="hidden" name="username" value="{{.Username}}"><input class="input compact" type="email" name="email" value="{{.Email}}" placeholder="No notification email"><button class="btn" type="submit">Save email</button></form></td><td>
<form class="useractions" method="post" action="/settings/users">
<input type="hidden" name="action" value="role">
<input type="hidden" name="username" value="{{.Username}}">
<label><span class="tiny">Role</span><select class="select compact" name="role">
<option value="viewer" {{if eq .Role "viewer"}}selected{{end}}>Viewer</option>
<option value="group_owner" {{if eq .Role "group_owner"}}selected{{end}}>Group owner</option>
<option value="operator" {{if eq .Role "operator"}}selected{{end}}>Operator</option>
<option value="admin" {{if eq .Role "admin"}}selected{{end}}>Admin</option>
</select></label><button class="btn" type="submit">Save role</button></form></td><td><form class="userforms" method="post" action="/settings/users"><input type="hidden" name="action" value="groups"><input type="hidden" name="username" value="{{.Username}}"><div class="membership-grid">{{range .GroupOptions}}<label class="membership-option"><input type="checkbox" name="group_ids" value="{{.ID}}" {{if .Selected}}checked{{end}}> <span>{{.Name}}</span></label>{{else}}<span class="tiny">No groups available</span>{{end}}</div><div class="membership-actions"><button class="btn" type="submit">Save selected</button><button class="btn" type="submit" onclick="this.form.querySelectorAll('input[name=group_ids]').forEach((item) => item.checked = false)">Remove all</button></div><span class="tiny">Check or uncheck several groups, then save once.</span></form><div class="chips" style="margin-top:8px">{{range .Groups}}<a class="chip" href="/groups/{{.ID}}">{{.Name}}</a>{{else}}<span class="tiny">No group access assigned</span>{{end}}</div></td>
<td>{{if .LastLoginAt}}{{.LastLoginAt}}{{else}}<span class="tiny">Never</span>{{end}}<div class="tiny">Timezone: {{.Timezone}}</div></td><td><span class="badge {{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</span>
<form method="post" action="/settings/users" style="margin-top:8px">
<input type="hidden" name="action" value="status">
<input type="hidden" name="username" value="{{.Username}}">
<input type="hidden" name="enabled" value="{{if .Enabled}}false{{else}}true{{end}}">
<button class="btn" type="submit">{{if .Enabled}}Disable{{else}}Enable{{end}}</button>
</form></td><td><form class="userforms" method="post" action="/settings/users">
<input type="hidden" name="action" value="reset_password">
<input type="hidden" name="username" value="{{.Username}}">
<input class="input" type="password" name="password" placeholder="New password" required minlength="16" autocomplete="new-password">
<input class="input" type="password" name="confirm_password" placeholder="Confirm password" required minlength="16" autocomplete="new-password">
<button class="btn" type="submit">Reset password</button>
</form><div class="tiny">Changed {{.PasswordChangedAt}}</div></td></tr>{{end}}
</tbody></table></div>{{else}}<div class="empty">No users configured.</div>{{end}}
</section></div></main></div></body></html>`

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

func (a *authManager) createUser(ctx context.Context, username, email, password, roleValue string, groupIDs []string) error {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	roleValue = strings.ToLower(strings.TrimSpace(roleValue))
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must contain between 3 and 64 characters")
	}
	if !validRole(roleValue) {
		return fmt.Errorf("role must be admin, operator, group_owner, or viewer")
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
	if roleValue != string(roleAdmin) && len(groupIDs) == 0 {
		return fmt.Errorf("at least one group is required for non-Admin users")
	}
	salt, err := newSalt()
	if err != nil {
		return err
	}
	if err := a.store.Create(ctx, sqlitestore.User{
		Username: username, Email: email, Role: roleValue, Enabled: true, GroupIDs: groupIDs, Timezone: "UTC",
		PasswordSalt: salt, PasswordHash: passwordDigest(password, a.pepper, salt),
	}); err != nil {
		return err
	}
	return a.syncGroupOwnership(ctx, username, roleValue, groupIDs)
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
	if userRole == string(roleGroupOwner) {
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
		err = a.createUser(r.Context(), target, r.FormValue("email"), r.FormValue("password"), r.FormValue("role"), r.Form["group_ids"])
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
		if existing.Role != string(roleAdmin) && len(groupIDs) == 0 {
			err = fmt.Errorf("at least one group is required for non-Admin users")
			break
		}
		err = a.store.UpdateGroups(r.Context(), target, groupIDs)
		if err == nil {
			err = a.syncGroupOwnership(r.Context(), target, existing.Role, groupIDs)
		}
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
		"status": "User status updated.", "email": "Notification email updated.", "groups": "Group access updated.", "reset_password": "Password reset.",
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

func (a *authManager) renderUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.store.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	current, _ := a.sessionUsername(r)
	allGroups, err := a.groupStore.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	view := usersPageData{
		Page: "settings-users", CurrentUser: current,
		Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error"),
		Users: make([]userSummary, 0, len(users)), Groups: allGroups,
	}
	for _, user := range users {
		summary := userSummary{
			Username: user.Username, Email: user.Email, Role: user.Role, Enabled: user.Enabled,
			GroupIDs: user.GroupIDs, Timezone: user.Timezone, LastLoginAt: user.LastLoginAt,
			PasswordChangedAt: user.PasswordChangedAt,
		}
		selected := map[string]bool{}
		for _, id := range user.GroupIDs {
			selected[id] = true
		}
		for _, group := range allGroups {
			summary.GroupOptions = append(summary.GroupOptions, userGroupOption{ID: group.ID, Name: group.Name, Selected: selected[group.ID]})
			if selected[group.ID] {
				summary.Groups = append(summary.Groups, group)
			}
		}
		view.Users = append(view.Users, summary)
		if user.Role == string(roleGroupOwner) {
			view.Owners = append(view.Owners, summary)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := usersPage.Execute(w, view); err != nil {
		slog.Error("render users page", "component", "http", "error", err)
	}
}
