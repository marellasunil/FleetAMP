package main

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type distributionPageView struct {
	Page                  string
	IsAdmin               bool
	Items                 []*distributions.Distribution
	Sections              []configs.SectionDefinition
	ReleaseByDistribution map[string]*distributions.DistributionRelease
	Message               string
	Error                 string
}

const distributionHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Collector Distributions · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.distribution-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.component-columns{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.section-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.distribution-card{display:grid;gap:14px}.managed-field{padding:10px;border:1px solid var(--line);border-radius:8px;background:#0a1626}@media(max-width:1050px){.distribution-grid,.component-columns,.section-options{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Distributions</div><div class="pagetitle">Collector Distribution Catalog</div><div class="subtitle">Consume immutable, organization-approved OpenTelemetry Collector build specifications.</div></div><div class="topactions">{{if .IsAdmin}}<a class="btn" href="/collector-distributions/build-plans">Build plans</a>{{end}}<span class="badge ok">Immutable catalog</span></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
{{if .IsAdmin}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Publish Collector distribution</div><div class="cardsub">Admin-only. This records a pinned build specification; it does not clone source or execute a build.</div></div><span class="badge off">No build execution</span></div><div class="cardbody"><form method="post" action="/collector-distributions"><div class="detailform"><label>Name<input class="input" name="name" required maxlength="120" placeholder="Organization Collector"></label><label>Collector version<input class="input" name="version" required maxlength="40" placeholder="0.140.0"></label><label>Pinned source tag or commit<input class="input" name="source_ref" required maxlength="160" placeholder="v0.140.0"></label><label>Published artifact URI (optional)<input class="input" name="artifact_uri" maxlength="500" placeholder="oci://registry.example/otelcol:0.140.0"></label><label>Artifact SHA-256 (optional)<input class="input code" name="artifact_sha256" maxlength="64" placeholder="64 hexadecimal characters"></label></div><label style="display:block;margin-top:16px"><strong>Organization baseline Collector YAML</strong><span class="tiny">Required. This immutable baseline is composed with group-editable overlays. Use secret references, never raw credentials.</span><textarea class="input code" name="baseline_config" rows="16" required placeholder="receivers:&#10;  otlp:&#10;    protocols:&#10;      grpc: {}&#10;exporters:&#10;  debug: {}&#10;service:&#10;  pipelines:&#10;    traces:&#10;      receivers: [otlp]&#10;      exporters: [debug]"></textarea></label><div class="component-columns" style="margin-top:16px"><label>Receivers<textarea class="input" name="receivers" rows="5" placeholder="otlp&#10;prometheus"></textarea></label><label>Processors<textarea class="input" name="processors" rows="5" placeholder="memory_limiter&#10;batch"></textarea></label><label>Exporters<textarea class="input" name="exporters" rows="5" placeholder="otlphttp"></textarea></label><label>Extensions<textarea class="input" name="extensions" rows="4" placeholder="health_check"></textarea></label><label>Connectors<textarea class="input" name="connectors" rows="4" placeholder="spanmetrics"></textarea></label></div><div class="section-options" style="margin-top:16px">{{range .Sections}}<div class="managed-field"><strong>{{.Title}}</strong><div class="tiny">{{.Description}}</div><label><input type="radio" name="section_{{.Key}}" value="locked" checked> Organization-managed</label><label><input type="radio" name="section_{{.Key}}" value="editable"> Group-editable</label></div>{{end}}</div><div class="notice" style="margin-top:16px">Publishing freezes the component inventory and configuration boundary. Corrections require a new distribution version.</div><div class="detailactions"><button class="btn primary" type="submit">Publish immutable specification</button></div></form></div></section>{{end}}
<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Published distributions</div><div class="cardsub">Authorized users can create validated, immutable group configuration previews from these versions.</div></div></div><div class="cardbody"><div class="distribution-grid">{{range .Items}}<article class="proposal-card distribution-card"><div><strong>{{.Name}}</strong><div class="tiny">Collector {{.CollectorVersion}} · {{.SourceRef}}</div></div><div class="chips"><span class="chip">{{len .Components.Receivers}} receivers</span><span class="chip">{{len .Components.Processors}} processors</span><span class="chip">{{len .Components.Exporters}} exporters</span><span class="chip">{{len .Components.Extensions}} extensions</span><span class="chip">{{len .Components.Connectors}} connectors</span></div><div><strong>Organization-managed sections</strong><div class="chips">{{range .LockedSections}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Group-editable sections</strong><div class="chips">{{range .EditableSections}}<span class="chip">{{.}}</span>{{else}}<span class="tiny">No user-editable sections</span>{{end}}</div></div>{{if .BaselineConfig}}<div><strong>Admin baseline</strong><div class="tiny code">SHA-256: {{.BaselineHash}}</div></div>{{else}}<div class="configerror">Legacy entry: publish a new version with an Admin baseline before group configuration.</div>{{end}}{{if .ArtifactURI}}<div class="tiny code">{{.ArtifactURI}}</div>{{end}}{{if .ArtifactSHA256}}<div class="tiny code">SHA-256: {{.ArtifactSHA256}}</div>{{end}}<div class="tiny code">Specification: {{.SpecHash}}</div><div class="tiny">Published by {{.PublishedBy}} · {{.PublishedAt}}</div><a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a></article>{{else}}<div class="empty">No Collector distributions have been published.</div>{{end}}</div></div></section></div></main></div></body></html>`

var distributionPage = template.Must(template.New("collector-distributions").Parse(strings.Replace(distributionHTML, `<a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a>`, `{{$release := index $.ReleaseByDistribution .ID}}{{if $release}}<div class="notice"><strong>Released package</strong><br><span class="tiny code">{{$release.ArtifactURI}}</span><br><span class="tiny code">SHA-256: {{$release.ArtifactSHA256}}</span></div>{{else if and .ArtifactURI .ArtifactSHA256}}<div class="notice"><strong>Legacy published package</strong><br><span class="tiny">Recorded before governed release evidence was introduced.</span></div>{{else}}<div class="tiny">Specification only · artifact release pending</div>{{end}}{{if or $release (and .ArtifactURI .ArtifactSHA256)}}<a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a>{{end}}`, 1)))

func registerDistributionRoutes(mux *http.ServeMux, store storage.DistributionStore, releaseStore storage.DistributionReleaseStore, validator *configs.Validator, auth *authManager) {
	mux.HandleFunc("/collector-distributions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collector-distributions" {
			http.NotFound(w, r)
			return
		}
		isAdmin := auth == nil || currentRole(auth, r) == roleAdmin
		if r.Method == http.MethodPost {
			if !isAdmin {
				http.Error(w, "administrator access required", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			locked, editable := sectionChoices(r)
			baseline := strings.TrimSpace(r.FormValue("baseline_config"))
			validation := validator.Validate(r.Context(), baseline)
			if !validation.Valid {
				renderDistributionPage(w, r, store, releaseStore, isAdmin, "", "Invalid organization baseline: "+validation.Error)
				return
			}
			distribution, err := distributions.New(r.FormValue("name"), r.FormValue("version"), r.FormValue("source_ref"), r.FormValue("artifact_uri"), r.FormValue("artifact_sha256"), distributions.ComponentSet{Receivers: splitLines(r.FormValue("receivers")), Processors: splitLines(r.FormValue("processors")), Exporters: splitLines(r.FormValue("exporters")), Extensions: splitLines(r.FormValue("extensions")), Connectors: splitLines(r.FormValue("connectors"))}, baseline, locked, editable, currentUsername(auth, r), time.Now())
			if err == nil {
				err = store.Publish(r.Context(), distribution)
			}
			if err != nil {
				renderDistributionPage(w, r, store, releaseStore, isAdmin, "", err.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions?message="+url.QueryEscape("Immutable Collector distribution published."), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderDistributionPage(w, r, store, releaseStore, isAdmin, r.URL.Query().Get("message"), "")
	})
}

func renderDistributionPage(w http.ResponseWriter, r *http.Request, store storage.DistributionStore, releaseStore storage.DistributionReleaseStore, isAdmin bool, message, pageError string) {
	items, err := store.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	releases, err := releaseStore.List(r.Context(), 1000)
	if err != nil {
		internalServerError(w, err)
		return
	}
	releaseByDistribution := map[string]*distributions.DistributionRelease{}
	for _, release := range releases {
		releaseByDistribution[release.DistributionID] = release
	}
	if !isAdmin {
		visible := make([]*distributions.Distribution, 0, len(items))
		for _, item := range items {
			if releaseByDistribution[item.ID] != nil || (item.ArtifactURI != "" && item.ArtifactSHA256 != "") {
				visible = append(visible, item)
			}
		}
		items = visible
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := distributionPage.Execute(w, distributionPageView{Page: "collector-distributions", IsAdmin: isAdmin, Items: items, Sections: configs.ConfigurationSectionDefinitions, ReleaseByDistribution: releaseByDistribution, Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}

func sectionChoices(r *http.Request) (locked, editable []string) {
	for _, definition := range configs.ConfigurationSectionDefinitions {
		if strings.TrimSpace(r.FormValue("section_"+definition.Key)) == "editable" {
			editable = append(editable, definition.Key)
		} else {
			locked = append(locked, definition.Key)
		}
	}
	return
}
