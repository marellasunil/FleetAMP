package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

type userSummary struct {
	Username          string
	Role              string
	Enabled           bool
	CreatedAt         time.Time
	PasswordChangedAt time.Time
}

type usersPageData struct {
	Page        string
	CurrentUser string
	Users       []userSummary
	Message     string
	Error       string
}

const sessionJS = `(() => {
  fetch("/api/v1/session", {credentials:"same-origin", headers:{Accept:"application/json"}})
    .then((response) => response.ok ? response.json() : Promise.reject())
    .then((session) => {
      const identity = document.getElementById("current-user");
      if (identity) {
        identity.textContent = session.username + " · " + session.role;
        identity.hidden = false;
      }
      const settings = document.getElementById("admin-settings-link");
      if (settings && session.role !== "admin") settings.hidden = true;
    })
    .catch(() => {});
})();`

const usersPageHTML = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Users · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.userforms{display:grid;gap:8px}.useractions{display:flex;gap:8px;flex-wrap:wrap;align-items:end}
.useractions label{display:grid;gap:5px}.compact{min-width:125px}
</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Settings / Users</div>
<div class="pagetitle">Users and roles</div>
<div class="subtitle">Admin-controlled local identities and FleetAMP authorization</div></div>
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
<label>Role<select class="select" name="role" required>
<option value="viewer">Viewer</option><option value="operator">Operator</option>
<option value="admin">Admin</option></select></label>
<label>Password<input class="input" type="password" name="password" required minlength="16" autocomplete="new-password"></label>
<label>Confirm password<input class="input" type="password" name="confirm_password" required minlength="16" autocomplete="new-password"></label>
<button class="btn primary" type="submit">Create user</button>
</form></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Managed users</div>
<div class="cardsub">{{len .Users}} local FleetAMP user(s)</div></div></div>
{{if .Users}}<div style="overflow:auto"><table><thead><tr>
<th>User</th><th>Role and access</th><th>Status</th><th>Password</th>
</tr></thead><tbody>{{range .Users}}<tr><td><strong>{{.Username}}</strong>
{{if eq .Username $.CurrentUser}}<div class="tiny">Current session</div>{{end}}
<div class="tiny">Created {{.CreatedAt}}</div></td><td>
<form class="useractions" method="post" action="/settings/users">
<input type="hidden" name="action" value="role">
<input type="hidden" name="username" value="{{.Username}}">
<label><span class="tiny">Role</span><select class="select compact" name="role">
<option value="viewer" {{if eq .Role "viewer"}}selected{{end}}>Viewer</option>
<option value="operator" {{if eq .Role "operator"}}selected{{end}}>Operator</option>
<option value="admin" {{if eq .Role "admin"}}selected{{end}}>Admin</option>
</select></label><button class="btn" type="submit">Save role</button></form></td>
<td><span class="badge {{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</span>
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

func validRole(value string) bool {
	switch role(value) {
	case roleAdmin, roleOperator, roleViewer:
		return true
	default:
		return false
	}
}

func (a *authManager) createUser(ctx context.Context, username, password, roleValue string) error {
	username = strings.TrimSpace(username)
	roleValue = strings.ToLower(strings.TrimSpace(roleValue))
	if len(username) < 3 || len(username) > 64 {
		return fmt.Errorf("username must contain between 3 and 64 characters")
	}
	if !validRole(roleValue) {
		return fmt.Errorf("role must be admin, operator, or viewer")
	}
	if len(password) < minimumAdminPassword {
		return fmt.Errorf("password must contain at least %d characters", minimumAdminPassword)
	}
	salt, err := newSalt()
	if err != nil {
		return err
	}
	return a.store.Create(ctx, sqlitestore.User{
		Username: username, Role: roleValue, Enabled: true,
		PasswordSalt: salt, PasswordHash: passwordDigest(password, a.pepper, salt),
	})
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
	writeJSON(w, http.StatusOK, map[string]string{"username": username, "role": string(principalRole)})
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
		err = a.createUser(r.Context(), target, r.FormValue("password"), r.FormValue("role"))
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
		err = a.store.UpdateRole(r.Context(), target, nextRole)
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
		"status": "User status updated.", "reset_password": "Password reset.",
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
	view := usersPageData{
		Page: "settings", CurrentUser: current,
		Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error"),
		Users: make([]userSummary, 0, len(users)),
	}
	for _, user := range users {
		view.Users = append(view.Users, userSummary{
			Username: user.Username, Role: user.Role, Enabled: user.Enabled,
			CreatedAt: user.CreatedAt, PasswordChangedAt: user.PasswordChangedAt,
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := usersPage.Execute(w, view); err != nil {
		slog.Error("render users page", "component", "http", "error", err)
	}
}
