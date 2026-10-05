package main

import (
	"html/template"
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

type runtimeProvidersView struct {
	Page      string
	Providers []runtimes.Descriptor
}

const runtimeProvidersHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Runtime Providers · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.runtime-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.runtime-card{display:flex;flex-direction:column}.runtime-card .cardbody{display:grid;gap:16px}.runtime-list{display:flex;flex-wrap:wrap;gap:6px}.capability-list{display:grid;gap:10px}.capability{display:grid;grid-template-columns:auto 1fr;gap:10px;align-items:start;padding:10px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.roadmap{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}@media(max-width:1100px){.runtime-grid,.roadmap{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Runtime Providers</div><div class="pagetitle">Telemetry Runtime Providers</div><div class="subtitle">Built-in, vendor-neutral runtime capabilities governed by FleetAMP core.</div></div><div class="topactions"><span class="badge off">Foundation</span></div></header><div class="content"><section class="card"><div class="cardhead"><div><div class="cardtitle">Built-in provider capability matrix</div><div class="cardsub">Supported means the workflow exists today. Planned capabilities are contracts for later integrations and do not perform cluster or repository changes.</div></div></div><div class="cardbody"><div class="runtime-grid">{{range .Providers}}<article class="card runtime-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub code">{{.Type}}</div></div><span class="badge ok">built-in</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Configuration</strong><div class="runtime-list" style="margin-top:7px"><span class="chip">{{.ConfigFormat}}</span></div></div><div><strong>Management modes</strong><div class="runtime-list" style="margin-top:7px">{{range .ManagementModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Deployment methods</strong><div class="runtime-list" style="margin-top:7px">{{range .DeploymentMethods}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Capabilities</strong><div class="capability-list" style="margin-top:7px">{{range .Capabilities}}<div class="capability"><span class="badge {{if eq .Support "supported"}}ok{{else}}off{{end}}">{{.Support}}</span><div><strong>{{.Name}}</strong><div class="tiny">{{.Detail}}</div></div></div>{{end}}</div></div></div></article>{{end}}</div></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Delivery sequence</div><div class="cardsub">Small, auditable milestones keep credentials and deployment authority out of the foundation.</div></div></div><div class="cardbody roadmap"><div class="component-item"><strong>1 · Runtime foundation</strong><div class="tiny">Provider catalog, capability reporting, and integration boundaries.</div><span class="badge ok" style="margin-top:8px">This milestone</span></div><div class="component-item"><strong>2 · Kubernetes renderer</strong><div class="tiny">Render and validate governed versions as OpenTelemetry Operator resources; no implicit apply.</div><span class="badge off" style="margin-top:8px">Next</span></div><div class="component-item"><strong>3 · Git connections</strong><div class="tiny">Repository identity, branch policy, pull/merge request handoff, status, and drift for GitHub, GitLab, and Azure DevOps.</div><span class="badge off" style="margin-top:8px">Planned</span></div></div></section></div></main></div></body></html>`

var runtimeProvidersPage = template.Must(template.New("runtime-providers").Parse(runtimeProvidersHTML))

func registerRuntimeProviderRoutes(mux *http.ServeMux, registry *runtimes.Registry) {
	mux.HandleFunc("/runtime-providers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime-providers" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := runtimeProvidersPage.Execute(w, runtimeProvidersView{Page: "runtime-providers", Providers: registry.List()}); err != nil {
			http.Error(w, "render runtime providers", http.StatusInternalServerError)
		}
	})
}
