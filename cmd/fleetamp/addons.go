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

const addonsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Add-ons · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.addon-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.addon-card .cardbody{display:grid;gap:14px}.capability-list{display:flex;flex-wrap:wrap;gap:7px}@media(max-width:900px){.addon-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Add-ons</div><div class="pagetitle">Add-ons</div><div class="subtitle">Discover optional integrations maintained outside FleetAMP core.</div></div><div class="topactions"><span class="badge off">Read-only catalog</span></div></header><div class="content"><section class="card"><div class="cardhead"><div><div class="cardtitle">External add-on catalog</div><div class="cardsub">Catalog entries are informational. This page cannot download, install, enable, or execute packages.</div></div><a class="btn" href="https://github.com/marellasunil/FleetAMP-Addons" target="_blank" rel="noreferrer">Open add-ons repository</a></div><div class="cardbody"><div class="notice">FleetAMP core remains vendor-neutral. Add-ons are not bundled and require an explicit future installation and security design.</div><div class="addon-grid" style="margin-top:16px">{{range .Entries}}<article class="card addon-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub code">{{.ID}}</div></div><span class="badge off">{{.Status}}</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Declared capabilities</strong><div class="capability-list" style="margin-top:8px">{{range .Capabilities}}<span class="chip">{{.}}</span>{{end}}</div></div><div class="notice"><strong>External and unbundled</strong><div class="tiny">{{.Notice}}</div></div><div class="detailactions"><a class="btn" href="{{.RepositoryURL}}" target="_blank" rel="noreferrer">Repository</a><a class="btn" href="{{.DocsURL}}" target="_blank" rel="noreferrer">Documentation</a><span class="btn" aria-disabled="true">Installation unavailable</span></div></div></article>{{else}}<div class="empty">No external add-ons are listed.</div>{{end}}</div></div></section></div></main></div></body></html>`

var addonsPage = template.Must(template.New("addons").Parse(addonsHTML))

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
			http.Error(w, "render add-ons", http.StatusInternalServerError)
		}
	})
}
