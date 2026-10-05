package main

import (
	"crypto/sha256"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/groups"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

const fleetAdoptionPageSize = 25

type fleetAdoptionRow struct {
	AgentUID, AgentName, Hostname, GroupID, GroupName string
	ConfigHash, PatternID, PatternName, Confidence    string
	State, Readiness, LastSeen                        string
	Score                                             int
	Attention                                         bool
}

type fleetAdoptionView struct {
	Page, Query, Status, GroupID, Error string
	Groups                              []*groups.Group
	Rows                                []fleetAdoptionRow
	Total, Aligned, Candidates          int
	Custom, Unassessed, Attention       int
	PageNumber, TotalPages              int
	PrevURL, NextURL                    string
}

const fleetAdoptionHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Fleet Adoption · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.summary{display:grid;grid-template-columns:repeat(6,minmax(120px,1fr));gap:12px;margin-bottom:16px}.metric{padding:14px;border:1px solid var(--line);border-radius:10px;background:#0a1626}.metric strong{display:block;font-size:24px}.filters{display:grid;grid-template-columns:minmax(220px,2fr) 1fr 1fr auto;gap:10px;align-items:end}.pagination{display:flex;align-items:center;justify-content:flex-end;gap:10px;padding:14px}.state{white-space:nowrap}@media(max-width:1100px){.summary{grid-template-columns:repeat(3,1fr)}.filters{grid-template-columns:1fr 1fr}}@media(max-width:700px){.summary,.filters{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Migration</div><div class="pagetitle">Fleet-wide Configuration Adoption</div><div class="subtitle">Track how existing Collector configurations align with governed Patterns across accessible ownership groups.</div></div><div class="topactions"><span class="badge ok">PR 6 · Fleet Adoption</span></div></header><div class="content"><nav class="tabs" aria-label="Migration stages"><a class="tab active" href="/migration/fleet-adoption">Fleet Adoption</a><a class="tab" href="/migration?tab=import">Import</a><a class="tab" href="/migration?tab=standardize">Standardization</a><a class="tab" href="/migration?tab=adoption">Collector Adoption</a><a class="tab" href="/migration?tab=pattern-match">Pattern Matching</a><a class="tab" href="/migration?tab=pattern-adopt">Adopt / Upgrade</a></nav>{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<div class="summary"><div class="metric"><span class="tiny">Accessible Collectors</span><strong>{{.Total}}</strong></div><div class="metric"><span class="tiny">Pattern aligned</span><strong>{{.Aligned}}</strong></div><div class="metric"><span class="tiny">Adoption candidates</span><strong>{{.Candidates}}</strong></div><div class="metric"><span class="tiny">Custom / unknown</span><strong>{{.Custom}}</strong></div><div class="metric"><span class="tiny">Unassessed</span><strong>{{.Unassessed}}</strong></div><div class="metric"><span class="tiny">Needs attention</span><strong>{{.Attention}}</strong></div></div><section class="card"><div class="cardhead"><div><div class="cardtitle">Collector adoption inventory</div><div class="cardsub">Read-only assessment of reported effective configuration. No Pattern is adopted or deployed from this page.</div></div></div><div class="cardbody"><form class="filters" method="get"><label>Search<input class="input" name="q" value="{{.Query}}" placeholder="Collector, host, group or Pattern"></label><label>State<select class="select" name="status"><option value="">All states</option><option value="aligned" {{if eq .Status "aligned"}}selected{{end}}>Pattern aligned</option><option value="candidate" {{if eq .Status "candidate"}}selected{{end}}>Adoption candidate</option><option value="custom" {{if eq .Status "custom"}}selected{{end}}>Custom / unknown</option><option value="unassessed" {{if eq .Status "unassessed"}}selected{{end}}>Unassessed</option><option value="invalid" {{if eq .Status "invalid"}}selected{{end}}>Invalid</option><option value="attention" {{if eq .Status "attention"}}selected{{end}}>Needs attention</option></select></label><label>Group<select class="select" name="group_id"><option value="">All accessible groups</option>{{range .Groups}}<option value="{{.ID}}" {{if eq .ID $.GroupID}}selected{{end}}>{{.Name}}</option>{{end}}</select></label><button class="btn primary" type="submit">Apply filters</button></form></div>{{if .Rows}}<div style="overflow:auto"><table><thead><tr><th>Collector</th><th>Group</th><th>Effective config</th><th>Best governed Pattern</th><th>State</th><th>Readiness</th><th>Last seen</th><th>Action</th></tr></thead><tbody>{{range .Rows}}<tr><td><a class="agentname" href="/agents/{{.AgentUID}}">{{.AgentName}}</a><div class="tiny">{{.Hostname}}</div><div class="tiny code">{{.AgentUID}}</div></td><td>{{if .GroupID}}<a href="/groups/{{.GroupID}}">{{.GroupName}}</a>{{else}}<span class="badge warn">Unassigned</span>{{end}}</td><td>{{if .ConfigHash}}<span class="code">{{.ConfigHash}}</span>{{else}}—{{end}}</td><td>{{if .PatternID}}<strong>{{.PatternName}}</strong><div class="tiny">{{.Score}}% · {{.Confidence}}</div>{{else}}—{{end}}</td><td class="state"><span class="badge {{if eq .State "aligned"}}ok{{else if or (eq .State "invalid") (eq .State "unassessed")}}warn{{else}}off{{end}}">{{.State}}</span></td><td>{{if .Attention}}<span class="badge warn">{{.Readiness}}</span>{{else}}<span class="badge ok">{{.Readiness}}</span>{{end}}</td><td>{{.LastSeen}}</td><td>{{if .GroupID}}<a class="btn" href="/migration?group_id={{.GroupID}}&amp;agent_uid={{.AgentUID}}">Review migration</a>{{else}}<span class="tiny">Assign a group first</span>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="cardbody tiny">No Collectors match these filters.</div>{{end}}<div class="pagination">{{if .PrevURL}}<a class="btn" href="{{.PrevURL}}">← Previous</a>{{end}}<span class="tiny">Page {{.PageNumber}} of {{.TotalPages}}</span>{{if .NextURL}}<a class="btn" href="{{.NextURL}}">Next →</a>{{end}}</div></section></div></main></div></body></html>`

var fleetAdoptionPage = template.Must(template.New("fleet-adoption").Parse(fleetAdoptionHTML))

func registerFleetAdoptionRoute(mux *http.ServeMux, groupStore storage.GroupStore, patternStore storage.DestinationProfileStore, agentStore *memory.AgentStore, adapter *fleetopamp.Adapter, auth *authManager) {
	mux.HandleFunc("/migration/fleet-adoption", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/migration/fleet-adoption" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		allGroups, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		principalRole := currentRole(auth, r)
		visibleGroups := groupsVisibleToUser(r.Context(), auth, currentUsername(auth, r), principalRole, allGroups)
		allAgents, err := agentStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		patterns, err := migrationCatalogPatterns(r.Context(), patternStore)
		if err != nil {
			internalServerError(w, err)
			return
		}
		rows := buildFleetAdoptionRows(migrationVisibleCollectors(allAgents, visibleGroups, principalRole), visibleGroups, patterns, adapter)
		view := fleetAdoptionView{Page: "migration", Groups: visibleGroups, Query: strings.TrimSpace(r.URL.Query().Get("q")), Status: strings.TrimSpace(r.URL.Query().Get("status")), GroupID: strings.TrimSpace(r.URL.Query().Get("group_id"))}
		view.Total = len(rows)
		for _, row := range rows {
			switch row.State {
			case "aligned":
				view.Aligned++
			case "candidate":
				view.Candidates++
			case "custom", "invalid":
				view.Custom++
			case "unassessed":
				view.Unassessed++
			}
			if row.Attention {
				view.Attention++
			}
		}
		rows = filterFleetAdoptionRows(rows, view.Query, view.Status, view.GroupID)
		view.PageNumber = positiveInt(r.URL.Query().Get("page"), 1)
		view.TotalPages = (len(rows) + fleetAdoptionPageSize - 1) / fleetAdoptionPageSize
		if view.TotalPages == 0 {
			view.TotalPages = 1
		}
		if view.PageNumber > view.TotalPages {
			view.PageNumber = view.TotalPages
		}
		start := (view.PageNumber - 1) * fleetAdoptionPageSize
		end := start + fleetAdoptionPageSize
		if end > len(rows) {
			end = len(rows)
		}
		view.Rows = rows[start:end]
		view.PrevURL, view.NextURL = fleetAdoptionPaginationURLs(r.URL.Query(), view.PageNumber, view.TotalPages)
		if err := fleetAdoptionPage.Execute(w, view); err != nil {
			internalServerError(w, err)
		}
	})
}

func buildFleetAdoptionRows(items []*agents.ManagedAgent, visibleGroups []*groups.Group, patterns []*blueprints.Pattern, adapter *fleetopamp.Adapter) []fleetAdoptionRow {
	rows := make([]fleetAdoptionRow, 0, len(items))
	for _, agent := range items {
		group := matchingVisibleGroup(agent, visibleGroups)
		effective := strings.TrimSpace(adapter.EffectiveConfig(agent.InstanceUID))
		row := fleetAdoptionRow{AgentUID: agent.InstanceUID, AgentName: agent.Name, Hostname: agent.Hostname, State: "unassessed", LastSeen: agent.LastSeen.UTC().Format("2006-01-02 15:04 UTC")}
		if row.AgentName == "" {
			row.AgentName = agent.InstanceUID
		}
		if group != nil {
			row.GroupID, row.GroupName = group.ID, group.Name
		}
		checks, ready := assessCollectorAdoption(agent, group, effective)
		row.Attention = !ready || !agent.Connected || !agent.Healthy
		row.Readiness = "Ready"
		if !agent.Connected {
			row.Readiness = "Offline"
		} else if !agent.Healthy || !ready {
			row.Readiness = "Needs attention"
		}
		_ = checks
		if effective != "" {
			sum := sha256.Sum256([]byte(effective))
			row.ConfigHash = fmt.Sprintf("%x", sum[:6])
			matches, err := matchCollectorPatterns(effective, patterns)
			switch {
			case err != nil:
				row.State = "invalid"
			case len(matches) == 0:
				row.State = "custom"
			default:
				best := matches[0]
				row.PatternID, row.PatternName, row.Score, row.Confidence = best.ID, best.Name, best.Score, best.Confidence
				if best.Score == 100 {
					row.State = "aligned"
				} else {
					row.State = "candidate"
				}
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].AgentName) < strings.ToLower(rows[j].AgentName) })
	return rows
}

func matchingVisibleGroup(agent *agents.ManagedAgent, visible []*groups.Group) *groups.Group {
	for _, group := range visible {
		if group.Enabled && groups.MatchesIdentity(group, agent) {
			return group
		}
	}
	return nil
}

func filterFleetAdoptionRows(rows []fleetAdoptionRow, query, status, groupID string) []fleetAdoptionRow {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]fleetAdoptionRow, 0, len(rows))
	for _, row := range rows {
		if groupID != "" && row.GroupID != groupID {
			continue
		}
		if status == "attention" && !row.Attention {
			continue
		}
		if status != "" && status != "attention" && row.State != status {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(strings.Join([]string{row.AgentName, row.Hostname, row.AgentUID, row.GroupName, row.PatternName}, " ")), query) {
			continue
		}
		result = append(result, row)
	}
	return result
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func fleetAdoptionPaginationURLs(values url.Values, page, total int) (string, string) {
	link := func(number int) string {
		next := url.Values{}
		for key, items := range values {
			for _, item := range items {
				next.Add(key, item)
			}
		}
		next.Set("page", strconv.Itoa(number))
		return "/migration/fleet-adoption?" + next.Encode()
	}
	previous, next := "", ""
	if page > 1 {
		previous = link(page - 1)
	}
	if page < total {
		next = link(page + 1)
	}
	return previous, next
}
