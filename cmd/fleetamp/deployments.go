package main

import (
	"html/template"
	"net/http"
)

type deploymentKind struct {
	Name        string
	Description string
	Status      string
}

type deploymentsView struct {
	Page  string
	Kinds []deploymentKind
}

const deploymentsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Deployments · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.delivery-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.delivery-flow{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.delivery-step{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}@media(max-width:1050px){.delivery-flow{grid-template-columns:1fr}.delivery-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Delivery / Deployments</div><div class="pagetitle">Deployments</div><div class="subtitle">One governed delivery path for configuration and component lifecycle changes.</div></div><div class="topactions"><a class="btn" href="/approvals">Open approvals</a><span class="badge off">Foundation</span></div></header><div class="content"><nav class="tabs" aria-label="Deployment views"><a class="tab active" href="/deployments">Requests</a><span class="tab">Active Rollouts <span class="soon">Planned</span></span><span class="tab">History <span class="soon">Planned</span></span><span class="tab">Rollbacks <span class="soon">Planned</span></span></nav><section class="card"><div class="cardhead"><div><div class="cardtitle">Supported request types</div><div class="cardsub">Configuration delivery works today. Component lifecycle requests define the safe contract for later Kubernetes execution.</div></div></div><div class="cardbody"><div class="delivery-grid">{{range .Kinds}}<article class="component-item"><div style="display:flex;justify-content:space-between;gap:12px"><strong>{{.Name}}</strong><span class="badge {{if eq .Status "Available"}}ok{{else}}off{{end}}">{{.Status}}</span></div><div class="tiny">{{.Description}}</div></article>{{end}}</div></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Governed delivery contract</div><div class="cardsub">Repository events and UI actions enter the same validation and approval boundary.</div></div></div><div class="cardbody delivery-flow"><div class="delivery-step"><strong>1 · Receive</strong><div class="tiny">UI, GitHub, GitLab or Azure DevOps proposes desired state.</div></div><div class="delivery-step"><strong>2 · Validate</strong><div class="tiny">Resolve group or labels, policy, compatibility and exact content hash.</div></div><div class="delivery-step"><strong>3 · Approve</strong><div class="tiny">Admin or group owner reviews the immutable request.</div></div><div class="delivery-step"><strong>4 · Roll out</strong><div class="tiny">Only the approved hash may reach eligible targets.</div></div><div class="delivery-step"><strong>5 · Verify</strong><div class="tiny">Record outcome, drift and rollback evidence in history and audit.</div></div></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Current workflow</div><div class="cardsub">Use the established group workspace until the consolidated request table is connected.</div></div></div><div class="cardbody detailactions"><a class="btn primary" href="/groups">Create from a group</a><a class="btn" href="/approvals">Review approval queue</a><a class="btn" href="/audit-log?tab=deployment">Open deployment audit</a></div></section></div></main></div></body></html>`

var deploymentsPage = template.Must(template.New("deployments").Parse(deploymentsHTML))

func registerDeploymentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/deployments", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deployments" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		view := deploymentsView{Page: "deployments", Kinds: []deploymentKind{
			{Name: "Configuration deployment", Description: "Deliver an immutable, validated Collector configuration version to approved group or label targets.", Status: "Available"},
			{Name: "Component installation", Description: "Install an approved OTel component through a future GitOps or cluster executor.", Status: "Planned"},
			{Name: "Component upgrade", Description: "Upgrade a discovered component only after compatibility validation and approval.", Status: "Planned"},
			{Name: "Component removal", Description: "Remove a component with target preview, dependency checks and explicit approval.", Status: "Planned"},
		}}
		if err := deploymentsPage.Execute(w, view); err != nil {
			http.Error(w, "render deployments", http.StatusInternalServerError)
		}
	})
}
