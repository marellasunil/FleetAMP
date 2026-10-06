package main

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type deploymentKind struct {
	Name        string
	Description string
	Status      string
}

type deploymentsView struct {
	Page     string
	Kinds    []deploymentKind
	Requests []lifecycleRequestView
	Error    string
	Message  string
}

type lifecycleRequestView struct {
	Request     *lifecycle.Request
	Validations []*lifecycle.Validation
}

const deploymentsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Deployments · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.delivery-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.delivery-flow{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.delivery-step{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.proposal-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.proposal-card{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.proposal-meta{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;margin-top:10px}.cardbody>form>.form-grid{grid-template-columns:minmax(0,760px);gap:18px}.cardbody>form>.form-grid>.field{display:grid;gap:7px}.cardbody>form>.form-grid>.field::after{color:var(--muted);font-size:13px;font-weight:400;line-height:1.45}.cardbody>form>.form-grid>.field:nth-child(1)::after{content:"Choose whether to install, upgrade, restart or remove the component."}.cardbody>form>.form-grid>.field:nth-child(2)::after{content:"Select the type of OpenTelemetry component this proposal will manage."}.cardbody>form>.form-grid>.field:nth-child(3)::after{content:"Identify the FleetAMP group whose eligible components will be resolved and validated."}.cardbody>form>.form-grid>.field:nth-child(4)::after{content:"Optionally narrow the group to components matching these exact labels."}.cardbody>form>.form-grid>.field:nth-child(5)::after{content:"Define how an approved proposal may eventually be delivered. This form performs no execution."}.cardbody>form>.form-grid>.field:nth-child(6)::after{content:"Required for upgrade, restart and removal proposals."}.cardbody>form>.form-grid>.field:nth-child(7)::after{content:"Required for install and upgrade proposals."}.cardbody>form>.form-grid>.field:nth-child(8)::after{content:"Explain the business or operational need so reviewers can make an informed decision."}@media(max-width:1050px){.delivery-flow,.delivery-grid,.proposal-grid,.proposal-meta{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / Deployments</div><div class="pagetitle">Deployments</div><div class="subtitle">One governed delivery path for configuration and component lifecycle changes.</div></div><div class="topactions"><a class="btn" href="/approvals">Open approvals</a><span class="badge off">Approval handoff</span></div></header><div class="content"><nav class="tabs" aria-label="Deployment views"><a class="tab active" href="/deployments">Requests</a><span class="tab">Active Rollouts <span class="soon">Planned</span></span><span class="tab">History <span class="soon">Planned</span></span><span class="tab">Rollbacks <span class="soon">Planned</span></span></nav>{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Create immutable component proposal</div><div class="cardsub">This freezes intent and target scope. It does not approve, install, upgrade, restart, or remove a component.</div></div><span class="badge off">No execution</span></div><div class="cardbody"><form method="post" action="/deployments"><div class="form-grid"><label class="field">Operation<select class="select" name="operation" required><option value="install">Install</option><option value="upgrade">Upgrade</option><option value="restart">Restart</option><option value="remove">Remove</option></select></label><label class="field">Component type<select class="select" name="component_type" required><option value="otel-collector">OpenTelemetry Collector</option><option value="otel-collector-kubernetes">Kubernetes OTel Collector</option><option value="otel-operator">OpenTelemetry Operator</option></select></label><label class="field">Target group ID<input class="input" name="group_id" maxlength="160" required placeholder="payments-production"></label><label class="field">Optional label selector<input class="input" name="label_selector" maxlength="500" placeholder="environment=prod,region=eu-west"></label><label class="field">Delivery method<select class="select" name="deployment_method" required><option value="gitops">GitOps</option><option value="kubernetes-api">Kubernetes API (future executor)</option><option value="systemd">Linux system service</option><option value="container-runtime">Container runtime</option><option value="manual-package">Existing package workflow</option></select></label><label class="field">Current version<input class="input" name="current_version" maxlength="80" placeholder="Required for upgrade, restart, remove"></label><label class="field">Desired version<input class="input" name="desired_version" maxlength="80" placeholder="Required for install and upgrade"></label><label class="field full">Reason<textarea class="input" name="reason" rows="3" maxlength="1000" required placeholder="Business and operational reason for this lifecycle change"></textarea></label></div><div class="notice" style="margin-top:12px">Install requires only a desired version. Upgrade requires different current and desired versions. Restart and removal require only the current version.</div><div class="detailactions"><button class="btn primary" type="submit">Create immutable proposal</button></div></form></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Component lifecycle proposals</div><div class="cardsub">Proposal and validation records are append-only. Only compatible snapshots can enter four-eyes approval.</div></div></div><div class="cardbody"><div class="proposal-grid">{{range .Requests}}<article class="proposal-card"><div style="display:flex;justify-content:space-between;gap:12px"><div><strong>{{.Request.Spec.Operation}} · {{.Request.Spec.ComponentType}}</strong><div class="tiny code">{{.Request.ID}}</div></div><span class="badge off">{{.Request.Status}}</span></div><div class="proposal-meta"><div><span class="tiny">Target</span><div>{{.Request.Spec.GroupID}}{{if .Request.Spec.LabelSelector}} · {{.Request.Spec.LabelSelector}}{{end}}</div></div><div><span class="tiny">Delivery</span><div>{{.Request.Spec.DeploymentMethod}}</div></div><div><span class="tiny">Versions</span><div>{{if .Request.Spec.CurrentVersion}}{{.Request.Spec.CurrentVersion}}{{else}}—{{end}} → {{if .Request.Spec.DesiredVersion}}{{.Request.Spec.DesiredVersion}}{{else}}—{{end}}</div></div><div><span class="tiny">Requested</span><div>{{.Request.RequestedBy}} · {{.Request.CreatedAt}}</div></div></div><div class="notice" style="margin-top:10px"><strong>Reason</strong><div class="tiny">{{.Request.Spec.Reason}}</div></div><div class="tiny code" style="margin-top:10px">Immutable SHA-256: {{.Request.SpecHash}}</div><form method="post" action="/deployments" class="detailactions"><input type="hidden" name="action" value="validate"><input type="hidden" name="request_id" value="{{.Request.ID}}"><button class="btn" type="submit">Resolve targets &amp; validate</button></form>{{range .Validations}}<div class="notice" style="margin-top:10px"><div style="display:flex;justify-content:space-between;gap:8px"><strong>Validation snapshot</strong><span class="badge {{if eq .Status "compatible"}}ok{{else if eq .Status "attention"}}warn{{else}}off{{end}}">{{.Status}}</span></div><div class="tiny">{{len .Targets}} resolved target(s) · {{len .Findings}} finding(s) · {{.ValidatedBy}} · {{.CreatedAt}}</div>{{range .Findings}}<div class="tiny"><strong>{{.Severity}}</strong> · {{.Code}}{{if .TargetID}} · {{.TargetID}}{{end}} — {{.Message}}</div>{{end}}<div class="tiny code">Result SHA-256: {{.ResultHash}}</div>{{if eq .Status "compatible"}}<form method="post" action="/deployments" class="detailform" style="margin-top:10px"><input type="hidden" name="action" value="submit_approval"><input type="hidden" name="request_id" value="{{.RequestID}}"><input type="hidden" name="validation_id" value="{{.ID}}"><label>Assigned reviewer<input class="input" name="assigned_reviewer" required maxlength="160" placeholder="Admin or group owner username"></label><label>Submission comment<input class="input" name="submission_comment" required maxlength="500" placeholder="Why this change should be approved"></label><button class="btn primary" type="submit">Submit for approval</button></form>{{end}}</div>{{end}}</article>{{else}}<div class="empty">No component lifecycle proposals have been created.</div>{{end}}</div></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Governed delivery contract</div><div class="cardsub">Repository events and UI actions enter the same validation and approval boundary.</div></div></div><div class="cardbody delivery-flow"><div class="delivery-step"><strong>1 · Propose</strong><div class="tiny">Freeze operation, component, target, method, versions and reason.</div></div><div class="delivery-step"><strong>2 · Validate</strong><div class="tiny">Resolve targets, policy, compatibility and exact specification hash.</div></div><div class="delivery-step"><strong>3 · Approve</strong><div class="tiny">Admin or group owner reviews the immutable request.</div></div><div class="delivery-step"><strong>4 · Roll out</strong><div class="tiny">Only the approved hash may reach an enabled executor.</div></div><div class="delivery-step"><strong>5 · Verify</strong><div class="tiny">Record health, outcome, drift and rollback evidence.</div></div></div></section></div></main></div></body></html>`

var deploymentsPage = template.Must(template.New("deployments").Parse(deploymentsHTML))

func registerDeploymentRoutes(mux *http.ServeMux, requestStore storage.ComponentLifecycleRequestStore, validationStore storage.ComponentLifecycleValidationStore, approvalStore storage.ComponentLifecycleApprovalStore, groupStore storage.GroupStore, agentStore storage.ManagedAgentStore, auth *authManager) {
	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deployments" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			renderDeployments(w, r, requestStore, validationStore, "")
		case http.MethodPost:
			if auth != nil && currentRole(auth, r) == roleViewer {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				renderDeployments(w, r, requestStore, validationStore, "Invalid proposal form.")
				return
			}
			if r.FormValue("action") == "validate" {
				request, err := requestStore.Get(r.Context(), strings.TrimSpace(r.FormValue("request_id")))
				if err != nil { renderDeployments(w, r, requestStore, validationStore, "Lifecycle proposal was not found."); return }
				group, err := groupStore.Get(r.Context(), request.Spec.GroupID)
				if err != nil { renderDeployments(w, r, requestStore, validationStore, "Target group was not found."); return }
				if auth != nil && !requireGroupAccess(w, r, auth, group) { return }
				inventory, err := agentStore.List(r.Context())
				if err != nil { renderDeployments(w, r, requestStore, validationStore, "Unable to resolve managed targets."); return }
				validator := currentUsername(auth, r); if validator == "" && auth == nil { validator = "test-user" }
				validation, err := lifecycle.ValidateRequest(request, group, inventory, validator)
				if err != nil { renderDeployments(w, r, requestStore, validationStore, err.Error()); return }
				if err := validationStore.Create(r.Context(), validation); err != nil { renderDeployments(w, r, requestStore, validationStore, "Unable to save validation snapshot."); return }
				http.Redirect(w, r, "/deployments?validated="+validation.ID, http.StatusSeeOther); return
			}
			if r.FormValue("action") == "submit_approval" {
				request, err := requestStore.Get(r.Context(), strings.TrimSpace(r.FormValue("request_id"))); if err != nil { renderDeployments(w,r,requestStore,validationStore,"Lifecycle proposal was not found."); return }
				validation, err := validationStore.Get(r.Context(), strings.TrimSpace(r.FormValue("validation_id"))); if err != nil { renderDeployments(w,r,requestStore,validationStore,"Validation snapshot was not found."); return }
				group, err := groupStore.Get(r.Context(), request.Spec.GroupID); if err != nil { renderDeployments(w,r,requestStore,validationStore,"Target group was not found."); return }
				if auth != nil && !requireGroupAccess(w,r,auth,group) { return }
				requester := currentUsername(auth,r); if requester=="" && auth==nil { requester="test-user" }
				reviewer := strings.TrimSpace(r.FormValue("assigned_reviewer")); if auth != nil { if err:=validateAssignedReviewer(r.Context(),auth,group,requester,reviewer); err!=nil { renderDeployments(w,r,requestStore,validationStore,err.Error()); return } }
				approval, err := lifecycle.NewApproval(request,validation,requester,reviewer,r.FormValue("submission_comment")); if err!=nil { renderDeployments(w,r,requestStore,validationStore,err.Error()); return }
				if err:=approvalStore.Create(r.Context(),approval); err!=nil { renderDeployments(w,r,requestStore,validationStore,"This validation snapshot has already been submitted for approval."); return }
				http.Redirect(w,r,"/component-approvals?submitted="+approval.ID,http.StatusSeeOther); return
			}
			requester := currentUsername(auth, r)
			if requester == "" && auth == nil {
				requester = "test-user"
			}
			request, err := lifecycle.NewRequest(lifecycle.Spec{
				Operation: lifecycle.Operation(r.FormValue("operation")), ComponentType: runtimes.Type(r.FormValue("component_type")),
				GroupID: r.FormValue("group_id"), LabelSelector: r.FormValue("label_selector"), DeploymentMethod: r.FormValue("deployment_method"),
				CurrentVersion: r.FormValue("current_version"), DesiredVersion: r.FormValue("desired_version"), Reason: r.FormValue("reason"),
			}, requester)
			if err != nil {
				renderDeployments(w, r, requestStore, validationStore, err.Error())
				return
			}
			if err := requestStore.Create(r.Context(), request); err != nil {
				renderDeployments(w, r, requestStore, validationStore, "Unable to save the lifecycle proposal.")
				return
			}
			http.Redirect(w, r, "/deployments?created="+request.ID, http.StatusSeeOther)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func renderDeployments(w http.ResponseWriter, r *http.Request, requestStore storage.ComponentLifecycleRequestStore, validationStore storage.ComponentLifecycleValidationStore, errorMessage string) {
	requests, err := requestStore.List(r.Context(), 100)
	if err != nil && errorMessage == "" {
		errorMessage = "Unable to load component lifecycle proposals."
	}
	message := ""
	if strings.TrimSpace(r.URL.Query().Get("created")) != "" {
		message = "Immutable component lifecycle proposal created. No action has been executed."
	} else if strings.TrimSpace(r.URL.Query().Get("validated")) != "" {
		message = "Immutable target and compatibility snapshot created. No action has been executed."
	}
	rows := make([]lifecycleRequestView, 0, len(requests))
	for _, request := range requests { validations, validationErr := validationStore.ListByRequest(r.Context(), request.ID); if validationErr != nil && errorMessage == "" { errorMessage = "Unable to load validation snapshots." }; rows = append(rows, lifecycleRequestView{Request: request, Validations: validations}) }
	view := deploymentsView{Page: "deployments", Requests: rows, Error: errorMessage, Message: message, Kinds: []deploymentKind{
		{Name: "Configuration deployment", Description: "Deliver an immutable, validated Collector configuration version to approved group or label targets.", Status: "Available"},
		{Name: "Component installation", Description: "Create an immutable installation proposal for later validation and approval.", Status: "Proposal"},
		{Name: "Component upgrade", Description: "Freeze current and desired versions before compatibility validation.", Status: "Proposal"},
		{Name: "Component restart", Description: "Propose a controlled restart without changing the component version.", Status: "Proposal"},
		{Name: "Component removal", Description: "Propose removal with an explicit target and operational reason.", Status: "Proposal"},
	}}
	if err := deploymentsPage.Execute(w, view); err != nil {
		http.Error(w, "render deployments", http.StatusInternalServerError)
	}
}
