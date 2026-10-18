package main

import (
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type distributionBuildPlanView struct {
	Page           string
	Distributions  []*distributions.Distribution
	Plans          []*distributions.BuildPlan
	Message, Error string
}

const distributionBuildPlanHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Collector Build Plans · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.build-layout{display:grid;grid-template-columns:minmax(0,1fr) minmax(360px,.8fr);gap:16px}.checks{display:grid;gap:8px}@media(max-width:1050px){.build-layout{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Distributions / Build plans</div><div class="pagetitle">Collector Build Plans</div><div class="subtitle">Freeze reproducible build intent before any external executor is enabled.</div></div><a class="btn" href="/collector-distributions">Distribution catalog</a></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<div class="build-layout"><section class="card"><div class="cardhead"><div><div class="cardtitle">Create immutable build plan</div><div class="cardsub">Admin-only. Planning performs no clone, command, container build, registry push or deployment.</div></div><span class="badge off">Execution disabled</span></div><div class="cardbody"><form method="post"><label>Published distribution<select class="select" name="distribution_id" required><option value="">Select distribution</option>{{range .Distributions}}<option value="{{.ID}}">{{.Name}} · Collector {{.CollectorVersion}}</option>{{end}}</select></label><fieldset class="checks" style="margin-top:14px"><legend>Build outputs</legend><label><input type="checkbox" name="outputs" value="oci-image"> OCI image</label><label><input type="checkbox" name="outputs" value="binary-archive"> Binary archive</label></fieldset><label style="display:block;margin-top:14px">Destination reference<input class="input code" name="destination_ref" required placeholder="oci://registry.example/platform/collector:0.140.0"></label><label style="display:block;margin-top:14px">Credential reference<input class="input code" name="credential_ref" required placeholder="secret://registry/platform"></label><div class="notice" style="margin-top:14px">Only references are stored. Never enter a token, password or private key.</div><div class="detailactions"><button class="btn primary" type="submit">Save immutable build plan</button></div></form></div></section><aside class="card"><div class="cardhead"><div><div class="cardtitle">Governance boundary</div><div class="cardsub">What this stage guarantees</div></div></div><div class="cardbody"><ul><li>Pinned official source reference</li><li>Frozen component inventory</li><li>Admin baseline evidence</li><li>Explicit output and destination</li><li>Credential reference only</li><li>No arbitrary command input</li></ul></div></aside></div><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Immutable plans</div><div class="cardsub">These records are ready for a later approval and executor stage.</div></div></div>{{if .Plans}}<div style="overflow:auto"><table><thead><tr><th>Plan</th><th>Distribution</th><th>Outputs</th><th>Destination</th><th>Requested</th><th>Status</th></tr></thead><tbody>{{range .Plans}}<tr><td class="code">{{.PlanHash}}</td><td class="code">{{.Manifest.DistributionSpecHash}}</td><td>{{range .Manifest.Outputs}}<span class="chip">{{.}}</span>{{end}}</td><td class="code">{{.Manifest.DestinationRef}}</td><td>{{.RequestedBy}}<div class="tiny">{{.RequestedAt}}</div></td><td><span class="badge off">{{.Status}}</span></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No build plans have been created.</div>{{end}}</section></div></main></div></body></html>`

var distributionBuildPlanPage = template.Must(template.New("distribution-build-plans").Parse(distributionBuildPlanHTML))

func registerDistributionBuildPlanRoutes(mux *http.ServeMux, distributionStore storage.DistributionStore, planStore storage.DistributionBuildPlanStore, auth *authManager) {
	mux.HandleFunc("/collector-distributions/build-plans", func(w http.ResponseWriter, r *http.Request) {
		if auth != nil && currentRole(auth, r) != roleAdmin {
			http.Error(w, "administrator access required", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			d, err := distributionStore.Get(r.Context(), strings.TrimSpace(r.FormValue("distribution_id")))
			if err == nil {
				var p *distributions.BuildPlan
				p, err = distributions.NewBuildPlan(d, r.Form["outputs"], r.FormValue("destination_ref"), r.FormValue("credential_ref"), currentUsername(auth, r), time.Now())
				if err == nil {
					err = planStore.Create(r.Context(), p)
				}
			}
			if err != nil {
				renderDistributionBuildPlans(w, r, distributionStore, planStore, "", err.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions/build-plans?message="+url.QueryEscape("Immutable build plan saved. No execution was performed."), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderDistributionBuildPlans(w, r, distributionStore, planStore, r.URL.Query().Get("message"), "")
	})
}
func renderDistributionBuildPlans(w http.ResponseWriter, r *http.Request, distributionStore storage.DistributionStore, planStore storage.DistributionBuildPlanStore, message, pageError string) {
	items, err := distributionStore.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	plans, err := planStore.List(r.Context(), 100)
	if err != nil {
		internalServerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := distributionBuildPlanPage.Execute(w, distributionBuildPlanView{Page: "collector-distributions", Distributions: items, Plans: plans, Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}
