package main

import (
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"strings"
)

type gitOpsPreviewsView struct {
	Page     string
	Plans    []gitOpsPlanView
	Previews []*lifecycle.GitOpsPreview
	Message  string
}

type gitOpsPlanView struct {
	Plan        *lifecycle.ExecutionPlan
	GroupID     string
	Connections []*integrations.Connection
}

const gitOpsPreviewsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>GitOps previews · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.gitops-plan-grid{display:grid;gap:12px}.gitops-plan{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.gitops-meta{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin:10px 0}@media(max-width:800px){.gitops-meta{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / GitOps Preview</div><div class="pagetitle">GitOps proposal previews</div><div class="subtitle">Bind an approved execution contract to an authorized repository boundary and render immutable evidence.</div></div><div class="topactions"><a class="btn" href="/component-approvals">Component approvals</a><span class="badge off">No Git write</span></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Prepare preview</div><div class="cardsub">Only enabled connections assigned to the proposal group are selectable. Repository paths are constrained beneath the connection’s allowed root.</div></div></div><div class="cardbody gitops-plan-grid">{{range .Plans}}<article class="gitops-plan"><strong>{{.Plan.Operation}} · {{.Plan.ComponentType}}</strong><div class="gitops-meta"><div><span class="tiny">Group</span><div>{{.GroupID}}</div></div><div><span class="tiny">Execution contract</span><div class="code tiny">{{.Plan.ID}}</div></div><div><span class="tiny">Plan SHA-256</span><div class="code tiny">{{.Plan.PlanHash}}</div></div></div>{{if .Connections}}<form method="post" action="/component-gitops-previews"><input type="hidden" name="plan_id" value="{{.Plan.ID}}"><label class="field">Enabled repository connection<select class="select" name="connection_id" required><option value="">Choose authorized connection…</option>{{range .Connections}}<option value="{{.ID}}">{{.Name}} · {{.Provider}} · {{.Organization}}{{if .Project}}/{{.Project}}{{end}}/{{.Repository}} · {{.Branch}}</option>{{end}}</select></label><div class="detailactions"><button class="btn primary" type="submit">Render immutable preview</button></div></form>{{else}}<div class="configerror">No enabled Git connection is authorized for group {{.GroupID}}.</div>{{end}}</article>{{else}}<div class="empty">No prepared GitOps execution contracts are available.</div>{{end}}</div></section>{{range .Previews}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">{{.RepositoryPath}}</div><div class="cardsub">Prepared by {{.PreparedBy}} · {{.CreatedAt}}</div></div><span class="badge ok">Immutable preview</span></div><div class="cardbody"><div class="gitops-meta"><div><span class="tiny">Connection</span><div>{{.Connection.Name}} · {{.Connection.Provider}}</div></div><div><span class="tiny">Repository</span><div>{{.Connection.Organization}}{{if .Connection.Project}}/{{.Connection.Project}}{{end}}/{{.Connection.Repository}}</div></div><div><span class="tiny">Branch / allowed root</span><div>{{.Connection.Branch}} · <span class="code">{{.Connection.AllowedRoot}}</span></div></div></div><div class="tiny code">Connection ID: {{.Connection.ID}}</div><div class="tiny code">Plan SHA-256: {{.PlanHash}}</div><div class="tiny code">Preview SHA-256: {{.PreviewHash}}</div><h3>Proposed diff</h3><pre>{{.Diff}}</pre>{{range .Files}}<h3>{{.Path}}</h3><pre>{{.Content}}</pre>{{end}}<div class="notice">No repository, branch, commit, pull request, cluster or component was changed.</div></div></section>{{else}}<div class="empty">No GitOps previews have been prepared.</div>{{end}}</div></main></div></body></html>`

var gitOpsPreviewsPage = template.Must(template.New("gitops-previews").Parse(gitOpsPreviewsHTML))

func registerGitOpsPreviewRoutes(mux *http.ServeMux, plans storage.ComponentLifecycleExecutionStore, requests storage.ComponentLifecycleRequestStore, previews storage.ComponentGitOpsPreviewStore, connections storage.IntegrationConnectionStore, auth *authManager) {
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
			connection, err := connections.Get(r.Context(), strings.TrimSpace(r.FormValue("connection_id")))
			if err != nil {
				http.Error(w, "Git connection unavailable", http.StatusUnprocessableEntity)
				return
			}
			preparedBy := currentUsername(auth, r)
			if preparedBy == "" && auth == nil {
				preparedBy = "test-admin"
			}
			preview, err := lifecycle.PrepareGitOpsPreview(plan, request, connection, preparedBy)
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
		connectionRows, err := connections.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		planViews := make([]gitOpsPlanView, 0, len(planRows))
		for _, plan := range planRows {
			if plan.ExecutorKind != lifecycle.ExecutorGitOps {
				continue
			}
			request, err := requests.Get(r.Context(), plan.RequestID)
			if err != nil {
				continue
			}
			eligible := make([]*integrations.Connection, 0)
			for _, connection := range connectionRows {
				if connection.Enabled && connectionAllowsGroup(connection, request.Spec.GroupID) {
					eligible = append(eligible, connection)
				}
			}
			planViews = append(planViews, gitOpsPlanView{Plan: plan, GroupID: request.Spec.GroupID, Connections: eligible})
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
		if err := gitOpsPreviewsPage.Execute(w, gitOpsPreviewsView{Page: "deployments", Plans: planViews, Previews: previewRows, Message: message}); err != nil {
			internalServerError(w, err)
		}
	})
}

func connectionAllowsGroup(connection *integrations.Connection, groupID string) bool {
	for _, allowed := range connection.GroupIDs {
		if allowed == groupID {
			return true
		}
	}
	return false
}
