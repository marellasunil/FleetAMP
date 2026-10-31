package main

import (
	"html/template"
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/addons"
)

type addonsView struct {
	Page    string
	Entries []addons.Entry
}

const addonsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Telemetry Integrations · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.integration-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.integration-card .cardbody{display:grid;gap:14px}.capability-list{display:flex;flex-wrap:wrap;gap:7px}@media(max-width:900px){.integration-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Configuration / Telemetry Integrations</div><div class="pagetitle">Telemetry Integrations</div><div class="subtitle">Review provider-neutral contracts for telemetry destinations and Collector interoperability.</div></div><div class="topactions"><span class="badge off">Read-only catalog</span></div></header><div class="content"><section class="page-description" aria-label="About this page"><div class="page-description-icon" aria-hidden="true">i</div><div><div class="page-description-title">About Telemetry Integrations</div><p>Review protocol-based integration contracts that any approved backend or Collector implementation can satisfy. FleetAMP describes signals, connection protocols and governance capabilities without embedding vendor names, endpoints or credentials; protected values remain in destination profiles and secret references.</p></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Integration capability catalog</div><div class="cardsub">These contracts are consumed by governed configuration and lifecycle workflows. They do not install vendor software or contact external services.</div></div></div><div class="cardbody"><div class="notice">Integrations are selected by capability and protocol. Organization-specific products are connected through protected configuration, not hard-coded into FleetAMP core.</div><div class="integration-grid" style="margin-top:16px">{{range .Entries}}<article class="card integration-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub">{{.Kind}} · <span class="code">{{.ID}}</span></div></div><span class="badge {{if eq .Status "available"}}ok{{else}}off{{end}}">{{.Status}}</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Protocols</strong><div class="capability-list" style="margin-top:8px">{{range .Protocols}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Signals</strong><div class="capability-list" style="margin-top:8px">{{range .Signals}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Governance capabilities</strong><div class="capability-list" style="margin-top:8px">{{range .Capabilities}}<span class="chip">{{.}}</span>{{end}}</div></div><div class="notice"><strong>Connection boundary</strong><div class="tiny">{{.Notice}}</div></div></div></article>{{else}}<div class="empty">No telemetry integration contracts are registered.</div>{{end}}</div></div></section></div></main></div></body></html>`

var addonsPage = template.Must(template.New("telemetry-integrations").Parse(addonsHTML))

func registerAddonRoutes(mux *http.ServeMux, catalog *addons.Catalog) {
	mux.HandleFunc("/addons", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/addons" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := addonsPage.Execute(w, addonsView{Page: "addons", Entries: catalog.List()}); err != nil {
			http.Error(w, "render telemetry integrations", http.StatusInternalServerError)
		}
	})
}
