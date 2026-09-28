package main

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type approvalDiffRow struct {
	OldText string
	NewText string
	Kind    string
}

type approvalItem struct {
	Request          *configs.GroupDeploymentRequest
	Base             *configs.Configuration
	Proposed         *configs.Configuration
	Diff             []approvalDiffRow
	CollectorCount   int
	EligibleCount    int
	ValidationStatus string
}

type approvalsView struct {
	Page   string
	Items  []approvalItem
	Status string
}

type approvalDetailView struct {
	Page      string
	Item      approvalItem
	CanReview bool
	CanCancel bool
}

func configurationLineDiff(oldContent, newContent string) []approvalDiffRow {
	oldLines := strings.Split(strings.TrimSuffix(oldContent, "\n"), "\n")
	newLines := strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
	if oldContent == "" {
		oldLines = nil
	}
	if newContent == "" {
		newLines = nil
	}
	lcs := make([][]int, len(oldLines)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(newLines)+1)
	}
	for i := len(oldLines) - 1; i >= 0; i-- {
		for j := len(newLines) - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	rows := make([]approvalDiffRow, 0, len(oldLines)+len(newLines))
	for i, j := 0, 0; i < len(oldLines) || j < len(newLines); {
		switch {
		case i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j]:
			rows = append(rows, approvalDiffRow{OldText: oldLines[i], NewText: newLines[j], Kind: "same"})
			i++
			j++
		case j < len(newLines) && (i == len(oldLines) || lcs[i][j+1] > lcs[i+1][j]):
			rows = append(rows, approvalDiffRow{NewText: newLines[j], Kind: "added"})
			j++
		default:
			rows = append(rows, approvalDiffRow{OldText: oldLines[i], Kind: "removed"})
			i++
		}
	}
	return rows
}

func buildApprovalItem(request *configs.GroupDeploymentRequest, configStore storage.ConfigurationStore, includeDiff bool, r *http.Request) (approvalItem, error) {
	item := approvalItem{
		Request:          request,
		CollectorCount:   len(request.Targets),
		ValidationStatus: "Passed at submission",
	}
	for _, target := range request.Targets {
		if target.Eligible {
			item.EligibleCount++
		}
	}
	proposed, err := configStore.Get(r.Context(), request.ConfigurationID)
	if err != nil {
		item.ValidationStatus = "Configuration unavailable"
		return item, err
	}
	item.Proposed = proposed
	if !includeDiff {
		return item, nil
	}
	if request.BaseConfigurationID != "" {
		item.Base, _ = configStore.Get(r.Context(), request.BaseConfigurationID)
	}
	baseContent := ""
	if item.Base != nil {
		baseContent = item.Base.Content
	}
	item.Diff = configurationLineDiff(baseContent, proposed.Content)
	return item, nil
}

func registerApprovalRoutes(mux *http.ServeMux, requestStore storage.GroupDeploymentRequestStore, configStore storage.ConfigurationStore, groupStore storage.GroupStore, notifier *approvalNotifier, auth *authManager) {
	mux.HandleFunc("/approvals", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/approvals" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		expireApprovalRequests(r.Context(), requestStore, groupStore, notifier)
		requests, err := requestStore.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		if status == "" {
			status = "active"
		}
		view := approvalsView{Page: "approvals", Status: status, Items: make([]approvalItem, 0, len(requests))}
		for _, request := range requests {
			if currentRole(auth, r) == roleGroupOwner {
				group, groupErr := groupStore.Get(r.Context(), request.GroupID)
				if groupErr != nil || (!isGroupOwner(group, currentUsername(auth, r)) && !strings.EqualFold(request.AssignedReviewer, currentUsername(auth, r))) {
					continue
				}
			}
			if !approvalStatusMatches(status, request.Status) {
				continue
			}
			item, itemErr := buildApprovalItem(request, configStore, false, r)
			if itemErr == nil {
				view.Items = append(view.Items, item)
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := approvalsPage.Execute(w, view); err != nil {
			internalServerError(w, err)
		}
	})

	mux.HandleFunc("/approvals/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/approvals/"), "/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		request, err := requestStore.Get(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		group, err := groupStore.Get(r.Context(), request.GroupID)
		if err != nil || !canAccessGroup(auth, r, group) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		item, err := buildApprovalItem(request, configStore, true, r)
		if err != nil {
			internalServerError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		username := currentUsername(auth, r)
		canReview := request.Status == configs.GroupDeploymentPendingApproval &&
			((request.AssignedReviewer == "" && currentRole(auth, r) == roleAdmin && !strings.EqualFold(username, request.RequestedBy)) || strings.EqualFold(username, request.AssignedReviewer))
		canCancel := request.Status == configs.GroupDeploymentPendingApproval &&
			(currentRole(auth, r) == roleAdmin || strings.EqualFold(username, request.RequestedBy))
		if err := approvalDetailPage.Execute(w, approvalDetailView{Page: "approvals", Item: item, CanReview: canReview, CanCancel: canCancel}); err != nil {
			internalServerError(w, err)
		}
	})
}

func approvalStatusMatches(filter string, status configs.GroupDeploymentRequestStatus) bool {
	switch filter {
	case "all":
		return true
	case "active":
		return status == configs.GroupDeploymentPendingApproval || status == configs.GroupDeploymentDeploying
	default:
		return string(status) == filter
	}
}

const approvalsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Approvals · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Approvals</div><div class="pagetitle">Deployment approvals</div><div class="subtitle">Review validated group changes, compare versions and approve or reject deployment.</div></div><div class="topactions"><span class="badge warn">Four-eyes approval</span></div></header><div class="content">
<section class="card"><div class="cardhead"><div><div class="cardtitle">Approval queue</div><div class="cardsub">Open a version to compare the previous and proposed configurations</div></div><form class="detailform" method="get" action="/approvals"><label>Status<select class="select" name="status" onchange="this.form.submit()"><option value="active" {{if eq .Status "active"}}selected{{end}}>Active</option><option value="expired" {{if eq .Status "expired"}}selected{{end}}>Expired</option><option value="completed" {{if eq .Status "completed"}}selected{{end}}>Completed</option><option value="rejected" {{if eq .Status "rejected"}}selected{{end}}>Rejected</option><option value="cancelled" {{if eq .Status "cancelled"}}selected{{end}}>Cancelled</option><option value="failed" {{if eq .Status "failed"}}selected{{end}}>Failed</option><option value="all" {{if eq .Status "all"}}selected{{end}}>All</option></select></label></form></div>
{{if .Items}}<div style="overflow:auto"><table><thead><tr><th>Group</th><th>Validation</th><th>Collectors</th><th>Requester</th><th>Configuration version</th><th>Request status</th><th>Submitted / expires</th></tr></thead><tbody>
{{range .Items}}<tr><td><strong>{{.Request.GroupName}}</strong><div class="tiny code">{{.Request.GroupID}}</div>{{if .Request.LabelSelector}}<div class="tiny">Labels: {{range $key,$value := .Request.LabelSelector}}<span class="code">{{$key}}={{$value}}</span> {{end}}</div>{{else}}<div class="tiny">All group members</div>{{end}}</td><td><span class="badge ok">✓ {{.ValidationStatus}}</span></td><td><strong>{{.CollectorCount}}</strong><div class="tiny">{{.EligibleCount}} ready at submission</div></td><td>{{.Request.RequestedBy}}<div class="tiny">Reviewer: {{if .Request.AssignedReviewer}}{{.Request.AssignedReviewer}}{{else}}Legacy unassigned{{end}}</div></td><td><a class="agentname" href="/approvals/{{.Request.ID}}">{{.Request.ConfigurationName}} · v{{.Request.ConfigurationVersion}}</a><div class="tiny">{{.Request.ChangeReason}}</div></td><td><span class="badge {{if eq .Request.Status "completed"}}ok{{else if eq .Request.Status "failed"}}warn{{else}}off{{end}}">{{.Request.Status}}</span></td><td>{{.Request.CreatedAt}}<div class="tiny">Expires {{.Request.ExpiresAt}}</div></td></tr>{{end}}
</tbody></table></div>{{else}}<div class="empty">No approval requests exist.</div>{{end}}</section>
</div></main></div></body></html>`

const approvalDetailHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Approval review · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.diff{display:grid;grid-template-columns:1fr 1fr;border:1px solid var(--line);border-radius:8px;overflow:hidden}.diffhead{padding:9px 12px;background:#0a1524;font-weight:700}.diffline{margin:0;padding:3px 9px;min-height:22px;white-space:pre-wrap;font:11px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;border-top:1px solid #17263a}.diffline.removed{background:#3a171d;color:#ff9dab}.diffline.added{background:#10342b;color:#77e7c0}.review{display:flex;gap:8px;align-items:end;flex-wrap:wrap}.review label{display:grid;gap:5px;flex:1;min-width:260px}@media(max-width:900px){.diff{grid-template-columns:1fr}.diffhead:nth-child(2){display:none}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Approvals / Review</div><div class="pagetitle">{{.Item.Request.GroupName}}</div><div class="subtitle">{{.Item.Request.ConfigurationName}} · version {{.Item.Request.ConfigurationVersion}}</div></div><div class="topactions"><a class="btn" href="/approvals">← Approval queue</a></div></header><div class="content">
<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Request summary</div><div class="cardsub">Review the scope and validation evidence before comparing changes</div></div><span class="badge {{if eq .Item.Request.Status "completed"}}ok{{else if eq .Item.Request.Status "failed"}}warn{{else}}off{{end}}">{{.Item.Request.Status}}</span></div><div class="cardbody"><div class="kv"><span>Group</span><span>{{.Item.Request.GroupName}}</span><span>Label target</span><span>{{if .Item.Request.LabelSelector}}{{range $key,$value := .Item.Request.LabelSelector}}<span class="code">{{$key}}={{$value}}</span> {{end}}{{else}}All group members{{end}}</span><span>Target semantics</span><span>Exact match · all labels required · immutable target snapshot</span><span>Requester</span><span>{{.Item.Request.RequestedBy}}</span><span>Assigned reviewer</span><span>{{if .Item.Request.AssignedReviewer}}{{.Item.Request.AssignedReviewer}}{{else}}Legacy unassigned request{{end}}</span><span>Change reason</span><span>{{if .Item.Request.ChangeReason}}{{.Item.Request.ChangeReason}}{{else}}Not recorded for this legacy request{{end}}</span><span>Validation</span><span class="green">✓ {{.Item.ValidationStatus}}</span><span>Collectors</span><span>{{.Item.CollectorCount}} snapshotted · {{.Item.EligibleCount}} ready</span><span>Submitted</span><span>{{.Item.Request.CreatedAt}}</span><span>Expires</span><span>{{.Item.Request.ExpiresAt}}</span><span>Proposed version</span><span>{{.Item.Request.ConfigurationName}} · v{{.Item.Request.ConfigurationVersion}}</span><span>Proposed hash</span><span class="code">{{.Item.Request.ConfigurationHash}}</span>{{if .Item.Base}}<span>Previous version</span><span>{{.Item.Base.Name}} · v{{.Item.Base.Version}}</span>{{else}}<span>Previous version</span><span>No common applied baseline</span>{{end}}</div></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Configuration comparison</div><div class="cardsub">Removed lines are red; added lines are green</div></div></div><div class="cardbody"><div class="diff"><div class="diffhead">Current approved / working configuration</div><div class="diffhead">Proposed validated configuration</div>{{range .Item.Diff}}<pre class="diffline {{if eq .Kind "removed"}}removed{{end}}">{{if .OldText}}{{.OldText}}{{else}} {{end}}</pre><pre class="diffline {{if eq .Kind "added"}}added{{end}}">{{if .NewText}}{{.NewText}}{{else}} {{end}}</pre>{{end}}</div>
{{if eq .Item.Request.Status "pending_approval"}}<div class="detailactions" style="margin-top:14px">{{if .CanReview}}<form class="review" method="post" action="/groups/{{.Item.Request.GroupID}}"><input type="hidden" name="action" value="approve_deployment"><input type="hidden" name="request_id" value="{{.Item.Request.ID}}"><label><span class="tiny">Review comment</span><input class="input" name="review_comment" maxlength="500"></label><button class="btn primary" type="submit">Approve & deploy</button></form><form class="review" method="post" action="/groups/{{.Item.Request.GroupID}}"><input type="hidden" name="action" value="reject_deployment"><input type="hidden" name="request_id" value="{{.Item.Request.ID}}"><label><span class="tiny">Rejection reason</span><input class="input" name="review_comment" required maxlength="500"></label><button class="btn" type="submit">Reject</button></form>{{end}}{{if .CanCancel}}<form class="review" method="post" action="/groups/{{.Item.Request.GroupID}}"><input type="hidden" name="action" value="cancel_deployment"><input type="hidden" name="request_id" value="{{.Item.Request.ID}}"><label><span class="tiny">Cancellation reason</span><input class="input" name="review_comment" required maxlength="500"></label><button class="btn" type="submit">Cancel request</button></form>{{end}}</div>{{else if .Item.Request.ReviewedBy}}<p class="tiny">Reviewed by {{.Item.Request.ReviewedBy}} at {{.Item.Request.ReviewedAt}}: {{.Item.Request.ReviewComment}}</p>{{end}}
</div></section></div></main></div></body></html>`

var approvalsPage = template.Must(template.New("approvals").Parse(approvalsHTML))
var approvalDetailPage = template.Must(template.New("approval-detail").Parse(approvalDetailHTML))
