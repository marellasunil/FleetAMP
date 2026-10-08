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
	Page       string
	Plans      []gitOpsPlanView
	Previews   []*lifecycle.GitOpsPreview
	Approvals  []*lifecycle.GitOpsPreviewApproval
	Executions []*lifecycle.GitOpsExecutionRequest
	Message    string
}

type gitOpsPlanView struct {
	Plan        *lifecycle.ExecutionPlan
	GroupID     string
	Connections []*integrations.Connection
}

const gitOpsPreviewsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>GitOps previews · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.gitops-plan-grid{display:grid;gap:12px}.gitops-plan{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.gitops-meta{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin:10px 0}@media(max-width:800px){.gitops-meta{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / GitOps Preview</div><div class="pagetitle">GitOps proposal previews</div><div class="subtitle">Bind an approved execution contract to an authorized repository boundary and render immutable evidence.</div></div><div class="topactions"><a class="btn" href="/component-approvals">Component approvals</a><span class="badge off">No Git write</span></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Prepare preview</div><div class="cardsub">Only enabled connections assigned to the proposal group are selectable. Repository paths are constrained beneath the connection’s allowed root.</div></div></div><div class="cardbody gitops-plan-grid">{{range .Plans}}<article class="gitops-plan"><strong>{{.Plan.Operation}} · {{.Plan.ComponentType}}</strong><div class="gitops-meta"><div><span class="tiny">Group</span><div>{{.GroupID}}</div></div><div><span class="tiny">Execution contract</span><div class="code tiny">{{.Plan.ID}}</div></div><div><span class="tiny">Plan SHA-256</span><div class="code tiny">{{.Plan.PlanHash}}</div></div></div>{{if .Connections}}<form method="post" action="/component-gitops-previews"><input type="hidden" name="plan_id" value="{{.Plan.ID}}"><label class="field">Enabled repository connection<select class="select" name="connection_id" required><option value="">Choose authorized connection…</option>{{range .Connections}}<option value="{{.ID}}">{{.Name}} · {{.Provider}} · {{.Organization}}{{if .Project}}/{{.Project}}{{end}}/{{.Repository}} · {{.Branch}}</option>{{end}}</select></label><div class="detailactions"><button class="btn primary" type="submit">Render immutable preview</button></div></form>{{else}}<div class="configerror">No enabled Git connection is authorized for group {{.GroupID}}.</div>{{end}}</article>{{else}}<div class="empty">No prepared GitOps execution contracts are available.</div>{{end}}</div></section>{{range .Previews}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">{{.RepositoryPath}}</div><div class="cardsub">Prepared by {{.PreparedBy}} · {{.CreatedAt}}</div></div><span class="badge ok">Immutable preview</span></div><div class="cardbody"><div class="gitops-meta"><div><span class="tiny">Connection</span><div>{{.Connection.Name}} · {{.Connection.Provider}}</div></div><div><span class="tiny">Repository</span><div>{{.Connection.Organization}}{{if .Connection.Project}}/{{.Connection.Project}}{{end}}/{{.Connection.Repository}}</div></div><div><span class="tiny">Branch / allowed root</span><div>{{.Connection.Branch}} · <span class="code">{{.Connection.AllowedRoot}}</span></div></div></div><div class="tiny code">Connection ID: {{.Connection.ID}}</div><div class="tiny code">Plan SHA-256: {{.PlanHash}}</div><div class="tiny code">Preview SHA-256: {{.PreviewHash}}</div><h3>Proposed diff</h3><pre>{{.Diff}}</pre>{{range .Files}}<h3>{{.Path}}</h3><pre>{{.Content}}</pre>{{end}}<form class="detailform" method="post" action="/component-gitops-previews"><input type="hidden" name="action" value="submit_preview_approval"><input type="hidden" name="preview_id" value="{{.ID}}"><label>Assigned reviewer<input class="input" name="assigned_reviewer" required maxlength="160" placeholder="A different administrator or group owner"></label><label>Submission comment<input class="input" name="submission_comment" required maxlength="500" placeholder="Why this exact repository change should be approved"></label><button class="btn primary" type="submit">Submit exact preview for approval</button></form><div class="notice">No repository, branch, commit, pull request, cluster or component was changed.</div></div></section>{{else}}<div class="empty">No GitOps previews have been prepared.</div>{{end}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">GitOps preview approvals</div><div class="cardsub">Every decision pins the preview, plan, connection, branch and repository path hashes.</div></div></div>{{if .Approvals}}<div style="overflow:auto"><table><thead><tr><th>Repository evidence</th><th>Preview hash</th><th>Submitter / reviewer</th><th>Status</th><th>Action</th></tr></thead><tbody>{{range .Approvals}}<tr><td><strong>{{.RepositoryPath}}</strong><div class="tiny">Branch: {{.Branch}}</div><div class="tiny code">Connection: {{.ConnectionID}}</div></td><td><div class="tiny code">Preview: {{.PreviewHash}}</div><div class="tiny code">Plan: {{.PlanHash}}</div></td><td>{{.SubmittedBy}}<div class="tiny">Reviewer: {{.AssignedReviewer}}</div><div class="tiny">{{.SubmissionComment}}</div></td><td><span class="badge {{if eq .Status "approved"}}ok{{else if eq .Status "pending_approval"}}warn{{else}}off{{end}}">{{.Status}}</span>{{if .ReviewedBy}}<div class="tiny">{{.ReviewedBy}} · {{.ReviewComment}}</div>{{end}}</td><td>{{if eq .Status "pending_approval"}}<form method="post" action="/component-gitops-previews"><input type="hidden" name="action" value="review_preview"><input type="hidden" name="approval_id" value="{{.ID}}"><input class="input" name="review_comment" maxlength="500" placeholder="Review comment"><div class="detailactions"><button class="btn primary" name="decision" value="approved">Approve exact preview</button><button class="btn" name="decision" value="rejected">Reject</button><button class="btn" name="decision" value="sent_back">Send back</button></div></form>{{else if eq .Status "approved"}}<form method="post" action="/component-gitops-previews"><input type="hidden" name="action" value="queue_execution"><input type="hidden" name="approval_id" value="{{.ID}}"><button class="btn primary" type="submit">Queue approved preview</button></form>{{else}}<span class="tiny">Decision recorded</span>{{end}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No GitOps previews have been submitted for approval.</div>{{end}}</section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Git execution outbox</div><div class="cardsub">Immutable execution requests waiting for a provider adapter. Queueing performs no repository write.</div></div><span class="badge off">Adapter disabled</span></div>{{if .Executions}}<div style="overflow:auto"><table><thead><tr><th>Repository path</th><th>Provider / mode</th><th>Evidence</th><th>Requested</th><th>Status</th></tr></thead><tbody>{{range .Executions}}<tr><td><strong>{{.RepositoryPath}}</strong><div class="tiny">Branch: {{.Branch}}</div></td><td>{{.Provider}}<div class="tiny">{{.Mode}}</div></td><td><div class="tiny code">Approval: {{.ApprovalID}}</div><div class="tiny code">Preview: {{.PreviewHash}}</div></td><td>{{.RequestedBy}}<div class="tiny">{{.CreatedAt}}</div></td><td><span class="badge warn">queued</span></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No approved GitOps executions are queued.</div>{{end}}</section></div></main></div></body></html>`

var gitOpsPreviewsPage = template.Must(template.New("gitops-previews").Parse(gitOpsPreviewsHTML))

func registerGitOpsPreviewRoutes(mux *http.ServeMux, plans storage.ComponentLifecycleExecutionStore, requests storage.ComponentLifecycleRequestStore, previews storage.ComponentGitOpsPreviewStore, previewApprovals storage.GitOpsPreviewApprovalStore, executions storage.GitOpsExecutionStore, connections storage.IntegrationConnectionStore, groupStore storage.GroupStore, auth *authManager) {
	mux.HandleFunc("/component-gitops-previews", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/component-gitops-previews" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			action := strings.TrimSpace(r.FormValue("action"))
			if action == "queue_execution" {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "invalid execution request", http.StatusBadRequest)
					return
				}
				approval, err := previewApprovals.Get(r.Context(), strings.TrimSpace(r.FormValue("approval_id")))
				if err != nil {
					http.NotFound(w, r)
					return
				}
				preview, err := previews.Get(r.Context(), approval.PreviewID)
				if err != nil || !canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				requestedBy := currentUsername(auth, r)
				if requestedBy == "" && auth == nil {
					requestedBy = "test-admin"
				}
				execution, event, err := lifecycle.NewGitOpsExecutionRequest(approval, preview, requestedBy)
				if err != nil {
					http.Error(w, err.Error(), http.StatusUnprocessableEntity)
					return
				}
				if err := executions.Create(r.Context(), execution, event); err != nil {
					http.Error(w, "approved preview already queued", http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/component-gitops-executions?queued="+execution.ID, http.StatusSeeOther)
				return
			}
			if action == "submit_preview_approval" {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "invalid approval submission", http.StatusBadRequest)
					return
				}
				preview, err := previews.Get(r.Context(), strings.TrimSpace(r.FormValue("preview_id")))
				if err != nil {
					http.NotFound(w, r)
					return
				}
				if !canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				submitter := currentUsername(auth, r)
				if submitter == "" && auth == nil {
					submitter = "test-admin"
				}
				reviewer := strings.TrimSpace(r.FormValue("assigned_reviewer"))
				if auth != nil {
					plan, err := plans.Get(r.Context(), preview.PlanID)
					if err != nil {
						http.Error(w, "preview evidence unavailable", http.StatusConflict)
						return
					}
					request, err := requests.Get(r.Context(), plan.RequestID)
					if err != nil {
						http.Error(w, "preview evidence unavailable", http.StatusConflict)
						return
					}
					group, err := groupStore.Get(r.Context(), request.Spec.GroupID)
					if err != nil || validateAssignedReviewer(r.Context(), auth, group, submitter, reviewer) != nil {
						http.Error(w, "eligible different reviewer required", http.StatusUnprocessableEntity)
						return
					}
				}
				approval, err := lifecycle.NewGitOpsPreviewApproval(preview, submitter, reviewer, r.FormValue("submission_comment"))
				if err != nil {
					http.Error(w, err.Error(), http.StatusUnprocessableEntity)
					return
				}
				if err := previewApprovals.Create(r.Context(), approval); err != nil {
					http.Error(w, "preview already submitted for approval", http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/component-gitops-previews?submitted="+approval.ID, http.StatusSeeOther)
				return
			}
			if action == "review_preview" {
				if err := r.ParseForm(); err != nil {
					http.Error(w, "invalid review", http.StatusBadRequest)
					return
				}
				approval, err := previewApprovals.Get(r.Context(), strings.TrimSpace(r.FormValue("approval_id")))
				if err != nil {
					http.NotFound(w, r)
					return
				}
				preview, err := previews.Get(r.Context(), approval.PreviewID)
				if err != nil || !canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				reviewer := currentUsername(auth, r)
				if reviewer == "" && auth == nil {
					reviewer = approval.AssignedReviewer
				}
				if !strings.EqualFold(reviewer, approval.AssignedReviewer) || strings.EqualFold(reviewer, approval.SubmittedBy) {
					http.Error(w, "assigned reviewer required", http.StatusForbidden)
					return
				}
				decision := lifecycle.ApprovalStatus(r.FormValue("decision"))
				if decision != lifecycle.ApprovalApproved && decision != lifecycle.ApprovalRejected && decision != lifecycle.ApprovalSentBack {
					http.Error(w, "invalid decision", http.StatusBadRequest)
					return
				}
				comment := strings.TrimSpace(r.FormValue("review_comment"))
				if decision != lifecycle.ApprovalApproved && comment == "" {
					http.Error(w, "review comment required", http.StatusUnprocessableEntity)
					return
				}
				if err := previewApprovals.Review(r.Context(), approval.ID, lifecycle.ApprovalPending, decision, reviewer, comment); err != nil {
					http.Error(w, "approval state changed", http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/component-gitops-previews?reviewed="+approval.ID, http.StatusSeeOther)
				return
			}
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
			if auth != nil {
				group, err := groupStore.Get(r.Context(), request.Spec.GroupID)
				if err != nil || !canAccessGroup(auth, r, group) {
					continue
				}
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
		visiblePreviews := make([]*lifecycle.GitOpsPreview, 0, len(previewRows))
		for _, preview := range previewRows {
			if canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
				visiblePreviews = append(visiblePreviews, preview)
			}
		}
		approvalRows, err := previewApprovals.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		visibleApprovals := make([]*lifecycle.GitOpsPreviewApproval, 0, len(approvalRows))
		for _, approval := range approvalRows {
			preview, err := previews.Get(r.Context(), approval.PreviewID)
			if err == nil && canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
				visibleApprovals = append(visibleApprovals, approval)
			}
		}
		executionRows, err := executions.List(r.Context(), 200)
		if err != nil {
			internalServerError(w, err)
			return
		}
		visibleExecutions := make([]*lifecycle.GitOpsExecutionRequest, 0, len(executionRows))
		for _, execution := range executionRows {
			preview, err := previews.Get(r.Context(), execution.PreviewID)
			if err == nil && canAccessGitOpsPreview(auth, r, preview, plans, requests, groupStore) {
				visibleExecutions = append(visibleExecutions, execution)
			}
		}
		message := ""
		if r.URL.Query().Get("prepared") != "" {
			message = "Immutable GitOps repository-change preview prepared. Nothing was written to Git."
		}
		if r.URL.Query().Get("submitted") != "" {
			message = "Exact GitOps preview hash submitted for four-eyes approval. Nothing was written to Git."
		}
		if r.URL.Query().Get("reviewed") != "" {
			message = "GitOps preview decision recorded against the immutable preview hash."
		}
		if r.URL.Query().Get("queued") != "" {
			message = "Approved immutable preview queued in the execution outbox. No Git write was performed."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := gitOpsPreviewsPage.Execute(w, gitOpsPreviewsView{Page: "deployments", Plans: planViews, Previews: visiblePreviews, Approvals: visibleApprovals, Executions: visibleExecutions, Message: message}); err != nil {
			internalServerError(w, err)
		}
	})
}

func canAccessGitOpsPreview(auth *authManager, r *http.Request, preview *lifecycle.GitOpsPreview, plans storage.ComponentLifecycleExecutionStore, requests storage.ComponentLifecycleRequestStore, groupStore storage.GroupStore) bool {
	if auth == nil {
		return true
	}
	plan, err := plans.Get(r.Context(), preview.PlanID)
	if err != nil {
		return false
	}
	request, err := requests.Get(r.Context(), plan.RequestID)
	if err != nil {
		return false
	}
	group, err := groupStore.Get(r.Context(), request.Spec.GroupID)
	return err == nil && canAccessGroup(auth, r, group)
}

func connectionAllowsGroup(connection *integrations.Connection, groupID string) bool {
	for _, allowed := range connection.GroupIDs {
		if allowed == groupID {
			return true
		}
	}
	return false
}
