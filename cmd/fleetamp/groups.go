// FleetAMP group, label, and group-management HTTP/UI handlers.
//
// Purpose:
//
//	Implements controlled Application/Environment/Place groups, agent group
//	assignment, managed labels, group CRUD, enable/disable lifecycle, and
//	membership-aware deletion protection for both REST and web UI routes.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type groupRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Selector    map[string]string `json:"selector"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

type groupListItem struct {
	Group       *groups.Group
	MemberCount int
}

type groupsView struct {
	Page  string
	Items []groupListItem
}
type groupDetailView struct {
	Page    string
	Group   *groups.Group
	Members []*agents.ManagedAgent
	Error   string
}

const maxManagedLabels = 5

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func registerGroupRoutes(mux *http.ServeMux, groupStore storage.GroupStore, agentStore *memory.AgentStore, dataDir string) {
	mux.HandleFunc("/agents/{uid}/group", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), strings.TrimSpace(r.FormValue("group_id")))
		if err != nil {
			http.Error(w, "group not found", http.StatusNotFound)
			return
		}
		if agent.GroupFields == nil {
			agent.GroupFields = map[string]string{}
		}
		for key, value := range group.Selector {
			agent.GroupFields[key] = value
		}
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})
	mux.HandleFunc("/agents/{uid}/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		labels, err := parseSelectorText(r.FormValue("labels"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		agent.Labels = labels
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})
	mux.HandleFunc("/agents/{uid}/label", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		agent, err := agentStore.Get(r.Context(), r.PathValue("uid"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		key, value := strings.TrimSpace(r.FormValue("key")), strings.TrimSpace(r.FormValue("value"))
		if key == "" || value == "" {
			http.Error(w, "label key and value are required", http.StatusBadRequest)
			return
		}
		if agent.Labels == nil {
			agent.Labels = map[string]string{}
		}
		if _, exists := agent.Labels[key]; !exists && len(agent.Labels) >= maxManagedLabels {
			http.Redirect(w, r, "/agents/"+agent.InstanceUID+"?error=Maximum+of+5+managed+labels+allowed", http.StatusSeeOther)
			return
		}
		agent.Labels[key] = value
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		http.Redirect(w, r, "/agents/"+agent.InstanceUID, http.StatusSeeOther)
	})

	mux.HandleFunc("/api/v1/agents/{uid}/labels", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uid := r.PathValue("uid")
		agent, err := agentStore.Get(r.Context(), uid)
		if err != nil {
			http.Error(w, "managed agent not found", http.StatusNotFound)
			return
		}
		var labels map[string]string
		if err := json.NewDecoder(r.Body).Decode(&labels); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		next := copyStringMap(agent.Labels)
		if r.Method == http.MethodPut || next == nil {
			next = map[string]string{}
		}
		for k, v := range labels {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			if k == "" {
				continue
			}
			if v == "" {
				delete(next, k)
			} else {
				next[k] = v
			}
		}
		if len(next) > maxManagedLabels {
			http.Error(w, "maximum of 5 managed labels allowed", http.StatusUnprocessableEntity)
			return
		}
		agent.Labels = next
		if err := agentStore.Upsert(r.Context(), agent); err != nil {
			internalServerError(w, err)
			return
		}
		if err := saveAgentSnapshot(r.Context(), agentStore, dataDir); err != nil {
			internalServerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, agent)
	})

	mux.HandleFunc("/api/v1/groups", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := groupStore.List(r.Context())
			if err != nil {
				internalServerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodPost:
			var req groupRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", 400)
				return
			}
			group, err := newValidatedGroup(req)
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if err := groupStore.Create(r.Context(), group); err != nil {
				http.Error(w, err.Error(), 409)
				return
			}
			writeJSON(w, http.StatusCreated, group)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/groups/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/groups/"), "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), parts[0])
		if err != nil {
			http.Error(w, "group not found", 404)
			return
		}
		if len(parts) == 2 && parts[1] == "members" && r.Method == http.MethodGet {
			members, err := membersForGroupIdentity(r.Context(), group, agentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, members)
			return
		}
		if len(parts) != 1 {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, group)
		case http.MethodPut:
			var req groupRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid JSON body", 400)
				return
			}
			selector := cleanSelector(req.Selector)
			if err := validateGroupSelector(selector); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			group.Name, group.Description, group.Selector, group.UpdatedAt = canonicalGroupName(selector), strings.TrimSpace(req.Description), selector, time.Now().UTC()
			if req.Enabled != nil {
				group.Enabled = *req.Enabled
			}
			if err := groupStore.Update(r.Context(), group); err != nil {
				internalServerError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, group)
		case http.MethodDelete:
			members, memberErr := membersForGroupIdentity(r.Context(), group, agentStore)
			if memberErr != nil {
				http.Error(w, memberErr.Error(), 500)
				return
			}
			if len(members) > 0 {
				http.Error(w, "group cannot be deleted while agents are assigned; unassign all agents first", http.StatusConflict)
				return
			}
			if err := groupStore.Delete(r.Context(), group.ID); err != nil {
				internalServerError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	registerGroupUI(mux, groupStore, agentStore)
}

func newValidatedGroup(req groupRequest) (*groups.Group, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Selector = cleanSelector(req.Selector)
	if err := validateGroupSelector(req.Selector); err != nil {
		return nil, err
	}
	req.Name = canonicalGroupName(req.Selector)
	return groups.New(req.Name, req.Description, req.Selector)
}

func validateGroupSelector(selector map[string]string) error {
	if strings.TrimSpace(selector["application"]) == "" {
		return errors.New("application selector is required")
	}
	if strings.TrimSpace(selector["environment"]) == "" {
		return errors.New("environment selector is required")
	}
	if strings.TrimSpace(selector["place"]) == "" {
		return errors.New("place selector is required")
	}
	return nil
}

func canonicalGroupName(selector map[string]string) string {
	parts := []string{selector["application"], selector["environment"], selector["place"]}
	for i, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		var b strings.Builder
		dash := false
		for _, r := range part {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				dash = false
			} else if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		parts[i] = strings.Trim(b.String(), "-")
	}
	return strings.Join(parts, "-")
}

func cleanSelector(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func membersForGroup(ctx context.Context, group *groups.Group, store *memory.AgentStore) ([]*agents.ManagedAgent, error) {
	if group != nil && !group.Enabled {
		return []*agents.ManagedAgent{}, nil
	}
	return membersByMatcher(ctx, group, store, groups.Matches)
}

func membersForGroupIdentity(ctx context.Context, group *groups.Group, store *memory.AgentStore) ([]*agents.ManagedAgent, error) {
	return membersByMatcher(ctx, group, store, groups.MatchesIdentity)
}

func membersByMatcher(ctx context.Context, group *groups.Group, store *memory.AgentStore, matcher func(*groups.Group, *agents.ManagedAgent) bool) ([]*agents.ManagedAgent, error) {
	all, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*agents.ManagedAgent, 0)
	for _, agent := range all {
		if matcher(group, agent) {
			result = append(result, agent)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func registerGroupUI(mux *http.ServeMux, groupStore storage.GroupStore, agentStore *memory.AgentStore) {
	mux.HandleFunc("/groups", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/groups" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			selector, err := parseGroupSelectorForm(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			group, err := newValidatedGroup(groupRequest{Selector: selector})
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := groupStore.Create(r.Context(), group); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		groupsList, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		view := groupsView{Page: "groups", Items: make([]groupListItem, 0, len(groupsList))}
		for _, group := range groupsList {
			members, _ := membersForGroupIdentity(r.Context(), group, agentStore)
			view.Items = append(view.Items, groupListItem{Group: group, MemberCount: len(members)})
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = groupsPage.Execute(w, view)
	})

	mux.HandleFunc("/groups/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/groups/"), "/")
		if id == "" {
			http.Redirect(w, r, "/groups", http.StatusPermanentRedirect)
			return
		}
		if strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			action := strings.TrimSpace(r.FormValue("action"))
			if action == "delete" {
				members, err := membersForGroupIdentity(r.Context(), group, agentStore)
				if err != nil {
					internalServerError(w, err)
					return
				}
				if len(members) > 0 {
					http.Redirect(w, r, "/groups/"+group.ID+"?error=Cannot+delete+group%3A+unassign+all+agents+from+this+group+first", http.StatusSeeOther)
					return
				}
				if err := groupStore.Delete(r.Context(), group.ID); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups", http.StatusSeeOther)
				return
			}
			if action == "disable" || action == "enable" {
				group.Enabled = action == "enable"
				group.UpdatedAt = time.Now().UTC()
				if err := groupStore.Update(r.Context(), group); err != nil {
					internalServerError(w, err)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
				return
			}
			if action == "update" {
				selector, err := parseGroupSelectorForm(r)
				if err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				group.Selector = selector
				group.Name = canonicalGroupName(selector)
				group.UpdatedAt = time.Now().UTC()
				if err := groupStore.Update(r.Context(), group); err != nil {
					http.Error(w, err.Error(), 409)
					return
				}
				http.Redirect(w, r, "/groups/"+group.ID, http.StatusSeeOther)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		members, err := membersForGroupIdentity(r.Context(), group, agentStore)
		if err != nil {
			internalServerError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = groupDetailPage.Execute(w, groupDetailView{Page: "groups", Group: group, Members: members, Error: r.URL.Query().Get("error")})
	})
}

func parseGroupSelectorForm(r *http.Request) (map[string]string, error) {
	selector := map[string]string{
		"application": strings.TrimSpace(r.FormValue("application")),
		"environment": strings.TrimSpace(r.FormValue("environment")),
		"place":       strings.TrimSpace(r.FormValue("place")),
	}
	if err := validateGroupSelector(selector); err != nil {
		return nil, err
	}
	return selector, nil
}

func parseSelectorText(input string) (map[string]string, error) {
	result := map[string]string{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 || strings.TrimSpace(pieces[0]) == "" || strings.TrimSpace(pieces[1]) == "" {
			return nil, errors.New("selector must use key=value pairs separated by commas")
		}
		result[strings.TrimSpace(pieces[0])] = strings.TrimSpace(pieces[1])
	}
	if len(result) == 0 {
		return nil, errors.New("at least one selector label is required")
	}
	return result, nil
}

var legacyGroupDetailPage = template.Must(template.New("group-detail").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>FleetAMP Group</title><style>
body{font-family:system-ui,sans-serif;background:#0b1220;color:#e5e7eb;margin:0;padding:32px;max-width:1400px}a{color:#93c5fd;text-decoration:none}.nav{display:flex;gap:16px;margin-bottom:22px}.card{background:#111827;border:1px solid #1f2937;border-radius:12px;padding:18px;margin-bottom:18px}table{width:100%;border-collapse:collapse}th,td{padding:12px;text-align:left;border-bottom:1px solid #1f2937}th{color:#93c5fd}code{color:#c4b5fd}.muted{color:#94a3b8}.ok{color:#86efac}.bad{color:#fca5a5}input{background:#0b1220;color:#e5e7eb;border:1px solid #334155;border-radius:7px;padding:9px;margin:4px;min-width:220px}button{padding:9px 14px;border-radius:7px;border:0;cursor:pointer}.danger{background:#7f1d1d;color:white}.warning{background:#422006;border:1px solid #92400e;color:#fde68a}.status{display:inline-block;padding:4px 9px;border-radius:999px;font-size:12px;font-weight:700}.enabled{background:#14532d;color:#bbf7d0}.disabled{background:#3f3f46;color:#d4d4d8}.actionrow{display:flex;gap:8px;flex-wrap:wrap}.iconaction{display:inline-flex;gap:7px;align-items:center}
</style></head><body><div class="nav"><a href="/agents">Agents</a><a href="/groups">Groups</a></div><h1>{{.Group.Name}}</h1><p>{{if .Group.Enabled}}<span class="status enabled">Enabled</span>{{else}}<span class="status disabled">Disabled</span>{{end}}</p>{{if .Error}}<section class="card warning"><strong>⚠️ {{.Error}}</strong></section>{{end}}
<section class="card"><h2>✏️ Edit group</h2><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="update"><label>Application<br><input name="application" value="{{index .Group.Selector "application"}}" required></label><label>Environment<br><input name="environment" value="{{index .Group.Selector "environment"}}" required></label><label>Place<br><input name="place" value="{{index .Group.Selector "place"}}" required></label><p class="muted">Group name is regenerated automatically from Application, Environment and Place.</p><button type="submit">💾 Update group</button></form><div class="actionrow" style="margin-top:14px"><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="{{if .Group.Enabled}}disable{{else}}enable{{end}}"><button type="submit" class="iconaction">{{if .Group.Enabled}}⏸️ Disable group{{else}}▶️ Enable group{{end}}</button></form><form method="post" action="/groups/{{.Group.ID}}"><input type="hidden" name="action" value="delete"><button class="danger iconaction" type="submit">🗑️ Delete group</button></form></div>{{if .Members}}<p class="muted" style="margin-top:12px">Delete protection is active: {{len .Members}} agent(s) currently match this group identity. Unassign them before deleting the group.</p>{{end}}</section>
<section class="card"><h2>Assigned agents ({{len .Members}})</h2>{{if .Members}}<table><thead><tr><th>Name</th><th>Type</th><th>Status</th><th>Health</th><th>Managed labels</th></tr></thead><tbody>{{range .Members}}<tr><td><a href="/agents/{{.InstanceUID}}">{{.Name}}</a></td><td>{{.Type}}</td><td>{{.Status}}</td><td class="{{if .Healthy}}ok{{else}}bad{{end}}">{{if .Healthy}}Healthy{{else}}Unhealthy{{end}}</td><td>{{range $k,$v := .Labels}}<code>{{$k}}={{$v}}</code> {{end}}</td></tr>{{end}}</tbody></table>{{else}}<p class="muted">No agents currently match this group identity.</p>{{end}}</section></body></html>`))
