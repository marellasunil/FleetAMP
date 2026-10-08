package main

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type gitOpsExecutionMonitorView struct {
	Page                string
	Executions          []gitOpsExecutionView
	Message             string
	GitHubWritesEnabled bool
	IsAdmin             bool
}

type gitOpsExecutionView struct {
	Request  *lifecycle.GitOpsExecutionRequest
	Events   []*lifecycle.GitOpsExecutionEvent
	Latest   *lifecycle.GitOpsExecutionEvent
	Attempt  int
	CanRetry bool
}

const gitOpsExecutionsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="refresh" content="10"><title>GitOps executions · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.execution-list{display:grid;gap:16px}.execution-card{border:1px solid var(--line);border-radius:10px;background:#0a1626;overflow:hidden}.execution-summary{display:grid;grid-template-columns:minmax(240px,2fr) repeat(3,minmax(140px,1fr));gap:14px;padding:16px}.execution-timeline{border-top:1px solid var(--line);padding:14px 16px}.execution-event{display:grid;grid-template-columns:120px minmax(180px,1fr) minmax(220px,2fr);gap:12px;padding:10px 0;border-bottom:1px solid var(--line)}.execution-event:last-child{border-bottom:0}.evidence{display:flex;flex-wrap:wrap;gap:6px;margin-top:7px}.evidence span{padding:3px 7px;border:1px solid var(--line);border-radius:6px}.status-failed{color:#fecaca}.status-succeeded{color:#bbf7d0}@media(max-width:900px){.execution-summary,.execution-event{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / GitOps Executions</div><div class="pagetitle">GitOps execution monitor</div><div class="subtitle">Append-only worker status and evidence for approved immutable previews.</div></div><div class="topactions"><a class="btn" href="/component-gitops-previews">Previews & approvals</a><a class="btn" href="/component-gitops-executions">Refresh</a>{{if .GitHubWritesEnabled}}<span class="badge warn">GitHub PR writes enabled</span>{{else}}<span class="badge off">Dry-run adapters</span>{{end}}</div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .GitHubWritesEnabled}}<div class="notice">Approved GitHub pull-request executions can create a FleetAMP branch, commit the approved files and open a pull request. GitLab and Azure DevOps remain dry-run.</div>{{else}}<div class="notice">The current provider adapters validate the workflow and record evidence only. They do not create commits, branches, pull requests or deployments.</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Execution outbox</div><div class="cardsub">This page refreshes every 10 seconds. Each timeline is immutable audit evidence.</div></div></div><div class="cardbody execution-list">{{range .Executions}}<article class="execution-card"><div class="execution-summary"><div><span class="tiny">Repository path</span><div><strong>{{.Request.RepositoryPath}}</strong></div><div class="tiny code">Execution: {{.Request.ID}}</div></div><div><span class="tiny">Provider / mode</span><div>{{.Request.Provider}}</div><div class="tiny">{{.Request.Mode}}</div></div><div><span class="tiny">Branch</span><div>{{.Request.Branch}}</div><div class="tiny">Requested by {{.Request.RequestedBy}}</div></div><div><span class="tiny">Latest status</span>{{if .Latest}}<div><span class="badge {{statusClass .Latest.Status}}">{{.Latest.Status}}</span></div><div class="tiny">{{.Latest.CreatedAt}}</div>{{else}}<span class="badge off">unknown</span>{{end}}</div></div><div class="execution-timeline"><strong>Status history</strong>{{range .Events}}<div class="execution-event"><div><span class="badge {{statusClass .Status}}">{{.Status}}</span><div class="tiny">{{.CreatedAt}}</div></div><div><strong>{{.Actor}}</strong><div class="tiny code">{{.ID}}</div></div><div>{{.Message}}{{if .Evidence}}<div class="evidence">{{range $key, $value := .Evidence}}<span class="tiny code">{{$key}}={{$value}}</span>{{end}}</div>{{end}}</div></div>{{end}}</div></article>{{else}}<div class="empty">No approved GitOps executions have been queued.</div>{{end}}</div></section></div></main></div></body></html>`

var gitOpsExecutionsPage = template.Must(template.New("gitops-executions").Funcs(template.FuncMap{
	"statusClass": gitOpsExecutionStatusClass,
}).Parse(strings.Replace(gitOpsExecutionsHTML,
	`</div></div><div class="execution-timeline"><strong>Status history</strong>`,
	`</div></div>{{if and $.IsAdmin .CanRetry}}<form class="detailform" style="margin:0 16px 16px" method="post" action="/component-gitops-executions"><input type="hidden" name="action" value="retry"><input type="hidden" name="execution_id" value="{{.Request.ID}}"><label>Retry reason<input class="input" name="reason" required maxlength="500" placeholder="Explain why this failed execution should be retried"></label><div class="tiny">Attempt {{.Attempt}} of 3 will reuse the same execution ID and Git branch.</div><button class="btn primary" type="submit">Retry failed execution</button></form>{{end}}<div class="execution-timeline"><strong>Status history</strong>`, 1)))

const maxGitOpsExecutionAttempts = 3

func registerGitOpsExecutionRoutes(mux *http.ServeMux, executions storage.GitOpsExecutionStore, previews storage.ComponentGitOpsPreviewStore, plans storage.ComponentLifecycleExecutionStore, requests storage.ComponentLifecycleRequestStore, groupStore storage.GroupStore, auth *authManager) {
	mux.HandleFunc("/component-gitops-executions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/component-gitops-executions" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if auth != nil && currentRole(auth, r) != roleAdmin {
				http.Error(w, "administrator access required", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil || r.FormValue("action") != "retry" {
				http.Error(w, "invalid retry request", http.StatusBadRequest)
				return
			}
			execution, err := executions.Get(r.Context(), strings.TrimSpace(r.FormValue("execution_id")))
			if err != nil {
				http.NotFound(w, r)
				return
			}
			preview, err := previews.Get(r.Context(), execution.PreviewID)
			if err != nil || !canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			reason := strings.TrimSpace(r.FormValue("reason"))
			if reason == "" || len(reason) > 500 {
				http.Error(w, "retry reason is required and must not exceed 500 characters", http.StatusUnprocessableEntity)
				return
			}
			events, err := executions.ListEvents(r.Context(), execution.ID)
			if err != nil {
				internalServerError(w, err)
				return
			}
			attempt := gitOpsExecutionAttempt(events) + 1
			actor := currentUsername(auth, r)
			if actor == "" && auth == nil {
				actor = "test-admin"
			}
			event, err := lifecycle.NewGitOpsExecutionRetryEvent(execution.ID, actor, reason, attempt)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			if err = executions.Retry(r.Context(), event, maxGitOpsExecutionAttempts); err != nil {
				http.Error(w, "execution cannot be retried in its current state or has reached the attempt limit", http.StatusConflict)
				return
			}
			http.Redirect(w, r, "/component-gitops-executions?retried="+execution.ID, http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, err := executions.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		visible := make([]gitOpsExecutionView, 0, len(rows))
		for _, execution := range rows {
			preview, err := previews.Get(r.Context(), execution.PreviewID)
			if err != nil || !canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
				continue
			}
			events, err := executions.ListEvents(r.Context(), execution.ID)
			if err != nil {
				internalServerError(w, err)
				return
			}
			view := gitOpsExecutionView{Request: execution, Events: events, Attempt: gitOpsExecutionAttempt(events) + 1}
			if len(events) > 0 {
				view.Latest = events[len(events)-1]
				view.CanRetry = view.Latest.Status == lifecycle.GitOpsExecutionFailed && view.Attempt <= maxGitOpsExecutionAttempts
			}
			visible = append(visible, view)
		}
		message := ""
		if strings.TrimSpace(r.URL.Query().Get("queued")) != "" {
			if boolEnv("FLEETAMP_GITOPS_GITHUB_WRITES_ENABLED") {
				message = "Approved preview queued for governed GitHub pull-request delivery."
			} else {
				message = "Approved preview queued. The dry-run worker will record evidence without writing to Git."
			}
		}
		if strings.TrimSpace(r.URL.Query().Get("retried")) != "" {
			message = "Failed execution queued for an administrator-approved retry."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := gitOpsExecutionsPage.Execute(w, gitOpsExecutionMonitorView{Page: "deployments", Executions: visible, Message: message, GitHubWritesEnabled: boolEnv("FLEETAMP_GITOPS_GITHUB_WRITES_ENABLED"), IsAdmin: auth == nil || currentRole(auth, r) == roleAdmin}); err != nil {
			internalServerError(w, err)
		}
	})
}

func gitOpsExecutionAttempt(events []*lifecycle.GitOpsExecutionEvent) int {
	attempts := 0
	for _, event := range events {
		if event.Status == lifecycle.GitOpsExecutionQueued {
			attempts++
		}
	}
	return attempts
}

func gitOpsExecutionStatusClass(status lifecycle.GitOpsExecutionStatus) string {
	switch status {
	case lifecycle.GitOpsExecutionSucceeded, lifecycle.GitOpsChangeMerged:
		return "ok status-succeeded"
	case lifecycle.GitOpsExecutionFailed, lifecycle.GitOpsChangeClosed:
		return "off status-failed"
	case lifecycle.GitOpsExecutionClaimed, lifecycle.GitOpsExecutionExecuting, lifecycle.GitOpsChangeOpen:
		return "warn"
	default:
		return "off"
	}
}
