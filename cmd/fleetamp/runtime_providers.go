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

const runtimeProvidersHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>OTel Components · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.runtime-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.runtime-card{display:flex;flex-direction:column}.runtime-card .cardbody{display:grid;gap:16px}.runtime-list{display:flex;flex-wrap:wrap;gap:6px}.capability-list{display:grid;gap:10px}.capability{display:grid;grid-template-columns:auto 1fr;gap:10px;align-items:start;padding:10px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.roadmap{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}@media(max-width:1100px){.runtime-grid,.roadmap{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Management / OTel Components</div><div class="pagetitle">OTel Components</div><div class="subtitle">Catalog the OpenTelemetry components FleetAMP can discover, govern and eventually deploy.</div></div><div class="topactions"><span class="badge off">Read-only foundation</span></div></header><div class="content"><nav class="tabs" aria-label="OTel component views"><a class="tab active" href="/otel-components">Catalog</a><span class="tab">Installed <span class="soon">Next</span></span><span class="tab">Compatibility <span class="soon">Planned</span></span><span class="tab">Installation Guides <span class="soon">Planned</span></span></nav><section class="card"><div class="cardhead"><div><div class="cardtitle">Built-in component catalog</div><div class="cardsub">Supported means the workflow exists today. Planned capabilities are contracts only and cannot change a cluster.</div></div></div><div class="cardbody"><div class="runtime-grid">{{range .Providers}}<article class="card runtime-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub code">{{.Type}}</div></div><span class="badge ok">built-in</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Roles</strong><div class="runtime-list" style="margin-top:7px">{{range .Roles}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Workload modes</strong><div class="runtime-list" style="margin-top:7px">{{range .WorkloadModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Configuration</strong><div class="runtime-list" style="margin-top:7px"><span class="chip">{{.ConfigFormat}}</span></div></div><div><strong>Management modes</strong><div class="runtime-list" style="margin-top:7px">{{range .ManagementModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Deployment methods</strong><div class="runtime-list" style="margin-top:7px">{{range .DeploymentMethods}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Capabilities</strong><div class="capability-list" style="margin-top:7px">{{range .Capabilities}}<div class="capability"><span class="badge {{if eq .Support "supported"}}ok{{else}}off{{end}}">{{.Support}}</span><div><strong>{{.Name}}</strong><div class="tiny">{{.Detail}}</div></div></div>{{end}}</div></div></div></article>{{end}}</div></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Safety boundary</div><div class="cardsub">Component installation remains intentionally disabled in this milestone.</div></div></div><div class="cardbody roadmap"><div class="component-item"><strong>Discover first</strong><div class="tiny">Inventory identity, version, platform and capabilities without changing workloads.</div></div><div class="component-item"><strong>Validate compatibility</strong><div class="tiny">Check component, Operator, CRD and FleetAMP support before a request is created.</div></div><div class="component-item"><strong>Approve before apply</strong><div class="tiny">Future installs and upgrades enter the same immutable approval workflow as configuration.</div></div></div></section></div></main></div></body></html>`

var runtimeProvidersPage = template.Must(template.New("runtime-providers").Parse(runtimeProvidersHTML))

func registerRuntimeProviderRoutes(mux *http.ServeMux, registry *runtimes.Registry) {
	mux.HandleFunc("/otel-components", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/otel-components" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := runtimeProvidersPage.Execute(w, runtimeProvidersView{Page: "otel-components", Providers: registry.List()}); err != nil {
			http.Error(w, "render OTel components", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/runtime-providers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime-providers" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/otel-components", http.StatusPermanentRedirect)
	})
}
