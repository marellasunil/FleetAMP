package main

import (
	"html/template"
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/integrations"
)

type integrationsView struct {
	Page      string
	Providers []integrations.Provider
}

const integrationsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Integrations · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.integration-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.integration-card .cardbody{display:grid;gap:15px}.integration-list{display:flex;flex-wrap:wrap;gap:7px}@media(max-width:1050px){.integration-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Administration / Integrations</div><div class="pagetitle">Git Integrations</div><div class="subtitle">Connect existing repositories without creating repositories or pipelines.</div></div><div class="topactions"><span class="badge off">Connection foundation</span></div></header><div class="content"><section class="card"><div class="cardhead"><div><div class="cardtitle">Choose your Git provider</div><div class="cardsub">This release defines connection and synchronization capabilities only. Credentials and provider API calls are not enabled.</div></div></div><div class="cardbody"><div class="notice">A future connection wizard will request the minimum permissions required by the organization-selected mode. Repository events will create immutable candidates; they will never bypass FleetAMP validation or approval.</div><div class="integration-grid" style="margin-top:16px">{{range .Providers}}<article class="card integration-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub code">{{.ID}}</div></div><span class="badge off">{{.Status}}</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Authentication options</strong><div class="integration-list" style="margin-top:8px">{{range .AuthOptions}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Capabilities</strong><div class="integration-list" style="margin-top:8px">{{range .Capabilities}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Synchronization modes</strong><div class="integration-list" style="margin-top:8px">{{range .SupportedModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div class="detailactions"><span class="btn primary" aria-disabled="true">Connect · Next milestone</span></div></div></article>{{end}}</div></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Governance boundary</div><div class="cardsub">The integration transports desired state; FleetAMP remains the deployment authority.</div></div></div><div class="cardbody"><div class="component-list"><div class="component-item"><strong>No repository creation</strong><div class="tiny">Users select an existing repository, branch, and allowed path.</div></div><div class="component-item"><strong>No pipeline creation</strong><div class="tiny">Signed webhooks or service hooks notify FleetAMP directly.</div></div><div class="component-item"><strong>Approval required</strong><div class="tiny">Every imported or written-back configuration uses the existing approval workflow.</div></div><div class="component-item"><strong>Controlled deployment</strong><div class="tiny">Only the exact approved content hash can be deployed to allowed groups or labels.</div></div></div></div></section></div></main></div></body></html>`

var integrationsPage = template.Must(template.New("integrations").Parse(integrationsHTML))

func registerIntegrationRoutes(mux *http.ServeMux, catalog *integrations.Catalog) {
	mux.HandleFunc("/settings/integrations", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/integrations" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := integrationsPage.Execute(w, integrationsView{Page: "settings-integrations", Providers: catalog.List()}); err != nil {
			http.Error(w, "render integrations", http.StatusInternalServerError)
		}
	})
}
