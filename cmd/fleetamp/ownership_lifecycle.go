package main

import (
	"html/template"
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ownershipLifecycleView struct {
	Page      string
	Requests  []*configs.MembershipChange
	Pending   int
	Executing int
	Completed int
	Attention int
}

const ownershipLifecycleHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ownership Lifecycle · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.ownership-flow{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.ownership-step{padding:13px;border:1px solid var(--line);border-radius:9px;background:#0a1626}@media(max-width:1000px){.ownership-flow{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Ownership</div><div class="pagetitle">Ownership Lifecycle</div><div class="subtitle">Govern Collector assignment, movement, release and configuration consequences across ownership groups.</div></div><div class="topactions"><a class="btn" href="/groups">Open groups</a><a class="btn" href="/approvals">Open approvals</a></div></header><div class="content"><div class="metrics"><div class="metric"><div class="metriclabel">Pending approval</div><div class="metricvalue amber">{{.Pending}}</div></div><div class="metric"><div class="metriclabel">Executing</div><div class="metricvalue purple">{{.Executing}}</div></div><div class="metric"><div class="metriclabel">Completed</div><div class="metricvalue green">{{.Completed}}</div></div><div class="metric"><div class="metriclabel">Needs attention</div><div class="metricvalue red">{{.Attention}}</div></div></div><section class="card"><div class="cardhead"><div><div class="cardtitle">Governed ownership contract</div><div class="cardsub">Group membership determines who may target a Collector; it never grants process lifecycle authority.</div></div></div><div class="cardbody ownership-flow"><div class="ownership-step"><strong>1 · Discover</strong><div class="tiny">Unassigned or dynamically matched Collector appears.</div></div><div class="ownership-step"><strong>2 · Propose</strong><div class="tiny">Assign, move or release with configuration impact.</div></div><div class="ownership-step"><strong>3 · Approve</strong><div class="tiny">Assigned reviewer checks both ownership boundaries.</div></div><div class="ownership-step"><strong>4 · Apply</strong><div class="tiny">Update controlled identity and selected configuration.</div></div><div class="ownership-step"><strong>5 · Verify</strong><div class="tiny">Record per-Collector results and rollback evidence.</div></div></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Ownership requests</div><div class="cardsub">Fleet-wide, append-only assignment, move and release history visible within your authorized groups.</div></div></div>{{if .Requests}}<div style="overflow:auto"><table><thead><tr><th>Operation</th><th>Ownership transition</th><th>Collectors</th><th>Configuration effect</th><th>Requester / reviewer</th><th>Status</th><th>Workspace</th></tr></thead><tbody>{{range .Requests}}<tr><td><strong>{{.Operation}}</strong><div class="tiny code">{{.ID}}</div></td><td>{{if .SourceGroupName}}{{.SourceGroupName}}{{else}}Unassigned{{end}} → {{if .TargetGroupName}}{{.TargetGroupName}}{{else}}Unassigned{{end}}</td><td>{{len .Targets}}{{range .Targets}}<div class="tiny">{{.AgentName}}</div>{{end}}</td><td>{{if .TargetConfigurationID}}Deploy selected destination configuration{{else if eq .RemovalPolicy "restore_pre_group"}}Restore pre-group configuration{{else}}Keep current configuration{{end}}</td><td>{{.RequestedBy}}<div class="tiny">Reviewer: {{.AssignedReviewer}}</div><div class="tiny">{{.Reason}}</div></td><td><span class="badge {{if eq .Status "completed"}}ok{{else if eq .Status "pending_approval"}}warn{{else}}off{{end}}">{{.Status}}</span>{{if .Error}}<div class="tiny red">{{.Error}}</div>{{end}}</td><td>{{if .SourceGroupID}}<a class="btn" href="/groups/{{.SourceGroupID}}/collectors">Open group</a>{{else if .TargetGroupID}}<a class="btn" href="/groups/{{.TargetGroupID}}/collectors">Open group</a>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No ownership lifecycle requests are visible. Start from a group’s Collector membership workspace.</div>{{end}}</section></div></main></div></body></html>`

var ownershipLifecyclePage = template.Must(template.New("ownership-lifecycle").Parse(ownershipLifecycleHTML))

func registerOwnershipLifecycleRoutes(mux *http.ServeMux, changes storage.MembershipChangeStore, groupStore storage.GroupStore, auth *authManager) {
	mux.HandleFunc("GET /ownership-lifecycle", func(w http.ResponseWriter, r *http.Request) {
		items, err := changes.List(r.Context(), 500)
		if err != nil {
			internalServerError(w, err)
			return
		}
		visible := make([]*configs.MembershipChange, 0, len(items))
		for _, item := range items {
			if ownershipChangeVisible(r, item, groupStore, auth) {
				visible = append(visible, item)
			}
		}
		view := ownershipLifecycleView{Page: "ownership-lifecycle", Requests: visible}
		for _, item := range visible {
			switch item.Status {
			case configs.MembershipPending:
				view.Pending++
			case configs.MembershipExecuting:
				view.Executing++
			case configs.MembershipCompleted:
				view.Completed++
			case configs.MembershipRejected, configs.MembershipFailed:
				view.Attention++
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ownershipLifecyclePage.Execute(w, view); err != nil {
			internalServerError(w, err)
		}
	})
}

func ownershipChangeVisible(r *http.Request, change *configs.MembershipChange, groupStore storage.GroupStore, auth *authManager) bool {
	if auth == nil || currentRole(auth, r) == roleAdmin {
		return true
	}
	for _, id := range []string{change.SourceGroupID, change.TargetGroupID} {
		if id == "" {
			continue
		}
		group, err := groupStore.Get(r.Context(), id)
		if err == nil && canAccessGroup(auth, r, group) {
			return true
		}
	}
	return false
}

