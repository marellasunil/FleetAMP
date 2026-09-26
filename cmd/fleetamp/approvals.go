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
	Request  *configs.GroupDeploymentRequest
	Base     *configs.Configuration
	Proposed *configs.Configuration
	Diff     []approvalDiffRow
}

type approvalsView struct {
	Page  string
	Items []approvalItem
	Error string
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

func registerApprovalRoutes(mux *http.ServeMux, requestStore storage.GroupDeploymentRequestStore, configStore storage.ConfigurationStore) {
	mux.HandleFunc("/approvals", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/approvals" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.NotFound(w, r)
			return
		}
		requests, err := requestStore.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		view := approvalsView{Page: "approvals", Items: make([]approvalItem, 0, len(requests))}
		for _, request := range requests {
			proposed, err := configStore.Get(r.Context(), request.ConfigurationID)
			if err != nil {
				continue
			}
			item := approvalItem{Request: request, Proposed: proposed}
			if request.BaseConfigurationID != "" {
				item.Base, _ = configStore.Get(r.Context(), request.BaseConfigurationID)
			}
			baseContent := ""
			if item.Base != nil {
				baseContent = item.Base.Content
			}
			item.Diff = configurationLineDiff(baseContent, proposed.Content)
			view.Items = append(view.Items, item)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := approvalsPage.Execute(w, view); err != nil {
			internalServerError(w, err)
		}
	})
}

const approvalsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Approvals · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.approval{margin-bottom:16px}.diff{display:grid;grid-template-columns:1fr 1fr;border:1px solid var(--line);border-radius:8px;overflow:hidden}.diffhead{padding:9px 12px;background:#0a1524;font-weight:700}.diffline{margin:0;padding:3px 9px;min-height:22px;white-space:pre-wrap;font:11px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;border-top:1px solid #17263a}.diffline.removed{background:#3a171d;color:#ff9dab}.diffline.added{background:#10342b;color:#77e7c0}.review{display:flex;gap:8px;align-items:end;flex-wrap:wrap}.review label{display:grid;gap:5px;flex:1;min-width:260px}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Approvals</div><div class="pagetitle">Deployment approvals</div><div class="subtitle">Admin review of validated, immutable group deployment requests</div></div><div class="topactions"><span class="badge warn">Four-eyes approval</span></div></header><div class="content">
{{if .Items}}{{range .Items}}
<section class="card approval">
<div class="cardhead"><div><div class="cardtitle">{{.Request.GroupName}} · {{.Request.ConfigurationName}} v{{.Request.ConfigurationVersion}}</div><div class="cardsub">Requested by {{.Request.RequestedBy}} · {{.Request.CreatedAt}} · {{len .Request.Targets}} snapshotted targets</div></div><span class="badge {{if eq .Request.Status "completed"}}ok{{else if eq .Request.Status "failed"}}warn{{else}}off{{end}}">{{.Request.Status}}</span></div>
<div class="cardbody">
<p class="tiny">Request <code>{{.Request.ID}}</code> · proposed hash <code>{{.Request.ConfigurationHash}}</code>{{if .Base}} · baseline {{.Base.Name}} v{{.Base.Version}}{{else}} · no common applied baseline{{end}}</p>
<div class="diff"><div class="diffhead">Current approved / working configuration</div><div class="diffhead">Proposed validated configuration</div>{{range .Diff}}<pre class="diffline {{if eq .Kind "removed"}}removed{{end}}">{{if .OldText}}{{.OldText}}{{else}} {{end}}</pre><pre class="diffline {{if eq .Kind "added"}}added{{end}}">{{if .NewText}}{{.NewText}}{{else}} {{end}}</pre>{{end}}</div>
{{if eq .Request.Status "pending_approval"}}<div class="detailactions" style="margin-top:14px"><form class="review" method="post" action="/groups/{{.Request.GroupID}}"><input type="hidden" name="action" value="approve_deployment"><input type="hidden" name="request_id" value="{{.Request.ID}}"><label><span class="tiny">Review comment</span><input class="input" name="review_comment" maxlength="500"></label><button class="btn primary" type="submit">Approve & deploy</button></form><form class="review" method="post" action="/groups/{{.Request.GroupID}}"><input type="hidden" name="action" value="reject_deployment"><input type="hidden" name="request_id" value="{{.Request.ID}}"><label><span class="tiny">Rejection reason</span><input class="input" name="review_comment" required maxlength="500"></label><button class="btn" type="submit">Reject</button></form></div>{{else if .Request.ReviewedBy}}<p class="tiny">Reviewed by {{.Request.ReviewedBy}} at {{.Request.ReviewedAt}}: {{.Request.ReviewComment}}</p>{{end}}
</div></section>
{{end}}{{else}}<section class="card"><div class="empty">No approval requests exist.</div></section>{{end}}
</div></main></div></body></html>`

var approvalsPage = template.Must(template.New("approvals").Parse(approvalsHTML))
