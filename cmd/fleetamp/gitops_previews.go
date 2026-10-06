package main

import (
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"strings"
)

type gitOpsPreviewsView struct {
	Page     string
	Plans    []*lifecycle.ExecutionPlan
	Previews []*lifecycle.GitOpsPreview
	Message  string
}

const gitOpsPreviewsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>GitOps previews · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / GitOps Preview</div><div class="pagetitle">GitOps proposal previews</div><div class="subtitle">Render immutable repository changes without writing to Git.</div></div><div class="topactions"><a class="btn" href="/component-approvals">Component approvals</a><span class="badge off">No Git write</span></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Prepare preview</div><div class="cardsub">Only prepared GitOps execution contracts are eligible. This creates immutable review evidence only.</div></div></div><div class="cardbody"><form method="post" action="/component-gitops-previews"><label class="field">Execution contract<select class="select" name="plan_id" required><option value="">Select a GitOps plan</option>{{range .Plans}}{{if eq .ExecutorKind "gitops"}}<option value="{{.ID}}">{{.Operation}} · {{.ComponentType}} · {{.PlanHash}}</option>{{end}}{{end}}</select></label><div class="detailactions"><button class="btn primary" type="submit">Render immutable preview</button></div></form></div></section>{{range .Previews}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">{{.RepositoryPath}}</div><div class="cardsub">Prepared by {{.PreparedBy}} · {{.CreatedAt}}</div></div><span class="badge ok">Preview only</span></div><div class="cardbody"><div class="tiny code">Plan SHA-256: {{.PlanHash}}</div><div class="tiny code">Preview SHA-256: {{.PreviewHash}}</div><h3>Proposed diff</h3><pre>{{.Diff}}</pre>{{range .Files}}<h3>{{.Path}}</h3><pre>{{.Content}}</pre>{{end}}<div class="notice">No repository, branch, commit, pull request, cluster or component was changed.</div></div></section>{{else}}<div class="empty">No GitOps previews have been prepared.</div>{{end}}</div></main></div></body></html>`

var gitOpsPreviewsPage = template.Must(template.New("gitops-previews").Parse(gitOpsPreviewsHTML))

func registerGitOpsPreviewRoutes(mux *http.ServeMux, plans storage.ComponentLifecycleExecutionStore, requests storage.ComponentLifecycleRequestStore, previews storage.ComponentGitOpsPreviewStore, auth *authManager) {
	mux.HandleFunc("/component-gitops-previews", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/component-gitops-previews" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if currentRole(auth, r) != roleAdmin {
				http.Error(w, "administrator access required", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid preview request", http.StatusBadRequest)
				return
			}
			plan, err := plans.Get(r.Context(), strings.TrimSpace(r.FormValue("plan_id")))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			request, err := requests.Get(r.Context(), plan.RequestID)
			if err != nil {
				http.Error(w, "proposal unavailable", http.StatusConflict)
				return
			}
			preparedBy := currentUsername(auth, r)
			if preparedBy == "" && auth == nil {
				preparedBy = "test-admin"
			}
			preview, err := lifecycle.PrepareGitOpsPreview(plan, request, preparedBy)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			if err := previews.Create(r.Context(), preview); err != nil {
				http.Error(w, "GitOps preview already prepared", http.StatusConflict)
				return
			}
			http.Redirect(w, r, "/component-gitops-previews?prepared="+preview.ID, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		planRows, err := plans.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		previewRows, err := previews.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		message := ""
		if r.URL.Query().Get("prepared") != "" {
			message = "Immutable GitOps repository-change preview prepared. Nothing was written to Git."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := gitOpsPreviewsPage.Execute(w, gitOpsPreviewsView{Page: "deployments", Plans: planRows, Previews: previewRows, Message: message}); err != nil {
			internalServerError(w, err)
		}
	})
}
