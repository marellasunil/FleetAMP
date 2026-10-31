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
	Stats                 distributionLifecycleStats
}

type distributionLifecycleStats struct {
	Specifications, BuildPlans, BuildsSucceeded, BuildsFailed int
	PromotionsPending, PublicationsQueued, PublicationsFailed int
	Releases, Previews, Handoffs                              int
}

type distributionLifecycleStores struct {
	BuildPlans            storage.DistributionBuildPlanStore
	BuildAttempts         storage.DistributionBuildAttemptStore
	Promotions            storage.DistributionArtifactPromotionStore
	PromotionReviews      storage.DistributionArtifactPromotionReviewStore
	Publications          storage.DistributionArtifactPublicationStore
	PublicationExecutions storage.DistributionArtifactPublicationExecutionStore
	Previews              storage.DistributionPreviewStore
	Handoffs              storage.DistributionPreviewHandoffStore
}

const distributionEditorJS = `(() => {
  const form = document.getElementById("distribution-editor-form");
  if (!form) return;
  const baseline = document.getElementById("distribution-baseline");
  const preview = document.getElementById("distribution-preview");
  const previewButton = document.getElementById("distribution-preview-button");
  const errorBox = document.getElementById("distribution-validation-error");
  const submitButton = form.querySelector('button[type="submit"]');
  const editors = Array.from(form.querySelectorAll("[data-section-key]"));
  const tabs = Array.from(form.querySelectorAll("[data-section-tab]"));
  const panels = Array.from(form.querySelectorAll("[data-section-panel]"));
  let validated = false;

  const selectTab = (key) => {
    tabs.forEach((tab) => {
      const active = tab.dataset.sectionTab === key;
      tab.classList.toggle("active", active);
      tab.setAttribute("aria-selected", String(active));
    });
    panels.forEach((panel) => panel.classList.toggle("active", panel.dataset.sectionPanel === key));
  };
  tabs.forEach((tab) => tab.addEventListener("click", () => selectTab(tab.dataset.sectionTab)));

  const clearValidation = () => {
    errorBox.hidden = true;
    errorBox.textContent = "";
    validated = false;
  };
  editors.forEach((editor) => editor.addEventListener("input", clearValidation));
  const sections = () => Object.fromEntries(editors.map((editor) => [editor.dataset.sectionKey, editor.value]));
  const composeAndValidate = async () => {
    const response = await fetch("/api/v1/configurations/sections/compose", {
      method: "POST",
      credentials: "same-origin",
      headers: {"Content-Type": "application/json", "Accept": "application/json"},
      body: JSON.stringify({baseline: "", sections: sections()})
    });
    const result = await response.json().catch(() => ({}));
    if (!response.ok || !result.validation || !result.validation.valid) {
      throw new Error((result.validation && result.validation.error) || "Collector configuration validation failed.");
    }
    baseline.value = result.content;
    preview.value = result.content;
  };
  const showError = (error) => {
    errorBox.textContent = error.message || "Collector configuration validation failed.";
    errorBox.hidden = false;
    errorBox.scrollIntoView({behavior: "smooth", block: "nearest"});
  };

  previewButton.addEventListener("click", async () => {
    clearValidation();
    previewButton.disabled = true;
    try {
      await composeAndValidate();
      selectTab("complete_preview");
    } catch (error) {
      showError(error);
    } finally {
      previewButton.disabled = false;
    }
  });
  form.addEventListener("submit", async (event) => {
    if (validated) return;
    event.preventDefault();
    clearValidation();
    submitButton.disabled = true;
    try {
      await composeAndValidate();
      validated = true;
      form.requestSubmit(submitButton);
    } catch (error) {
      showError(error);
      submitButton.disabled = false;
    }
  });
})();`

const distributionHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Collector Packages · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.distribution-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.distribution-card{display:grid;gap:14px}.lifecycle-stats{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px;margin-bottom:16px}.lifecycle-stat{padding:14px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.lifecycle-value{font-size:24px;font-weight:700}.lifecycle-label{color:var(--muted);font-size:12px}.package-form{display:grid;gap:18px}.package-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px 18px}.package-field{display:grid;gap:7px;align-content:start;color:var(--muted);font-size:12px}.package-field.full{grid-column:1/-1}.package-field-help{color:var(--muted);font-size:12px;line-height:1.45}.package-panel{padding:16px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.package-panel-title{margin-bottom:4px;font-weight:700;color:var(--text)}.package-panel-subtitle{margin-bottom:14px;color:var(--muted);font-size:12px;line-height:1.45}.package-yaml{width:100%;min-height:340px;resize:vertical;background:#07111e;border:1px solid #29405f;border-radius:8px;padding:13px;color:#d8e5ff;font:12px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace}.component-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px 18px}.component-fields textarea{min-height:105px;resize:vertical}.section-options{display:grid;gap:10px}.managed-field{display:grid;grid-template-columns:minmax(180px,.65fr) minmax(220px,1fr);gap:14px;align-items:center;padding:13px 14px;border:1px solid var(--line);border-radius:8px;background:#07111e}.ownership-options{display:flex;gap:18px;flex-wrap:wrap}.ownership-options label{display:flex;align-items:center;gap:7px;color:var(--text);font-size:13px}.package-actions{display:flex;justify-content:flex-end;gap:8px;flex-wrap:wrap;padding-top:2px}@media(max-width:1050px){.distribution-grid{grid-template-columns:1fr}.lifecycle-stats{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.package-fields,.component-fields{grid-template-columns:1fr}.package-field.full{grid-column:auto}.managed-field{grid-template-columns:1fr}.package-actions{justify-content:flex-start}}@media(max-width:600px){.lifecycle-stats{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Collector Packages</div><div class="pagetitle">Collector Packages</div><div class="subtitle">Select approved Collector software packages and configure them for authorized groups.</div></div><div class="topactions">{{if .IsAdmin}}<a class="btn" href="/collector-distributions/build-plans">Build &amp; release</a>{{end}}<span class="badge ok">Approved packages</span></div></header><div class="content"><section class="page-description" aria-label="About this page"><div class="page-description-icon" aria-hidden="true">i</div><div><div class="page-description-title">About Collector Packages</div><p>A Collector package is the approved software artifact FleetAMP can build and release: its Collector version, compiled receivers, processors, exporters, extensions and connectors. It is not a configuration template or blueprint. Configuration Builder defines runtime pipelines; this page selects the software package those configurations run on.</p></div></section>{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
<section class="lifecycle-stats" aria-label="Collector package summary"><div class="lifecycle-stat"><div class="lifecycle-value">{{.Stats.Specifications}}</div><div class="lifecycle-label">Package definitions</div></div><div class="lifecycle-stat"><div class="lifecycle-value">{{.Stats.Releases}}</div><div class="lifecycle-label">Ready packages</div></div><div class="lifecycle-stat"><div class="lifecycle-value">{{.Stats.Previews}}</div><div class="lifecycle-label">Group configurations</div><div class="tiny">{{.Stats.Handoffs}} submitted for approval</div></div></section><details class="card" style="margin-bottom:16px"><summary class="cardhead"><div><div class="cardtitle">Build &amp; release status</div><div class="cardsub">Advanced package pipeline details</div></div><span class="badge off">{{.Stats.BuildPlans}} build plans</span></summary><div class="cardbody"><div class="chips"><span class="chip">{{.Stats.BuildsSucceeded}} successful builds</span><span class="chip">{{.Stats.BuildsFailed}} failed attempts</span><span class="chip">{{.Stats.PromotionsPending}} promotions awaiting review</span><span class="chip">{{.Stats.PublicationsQueued}} publications queued</span><span class="chip">{{.Stats.PublicationsFailed}} publications failed</span></div></div></details>
{{if .IsAdmin}}<section class="card wide" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Publish Collector distribution</div><div class="cardsub">Create an immutable, organization-approved Collector package definition</div></div><span class="badge off">Admin only</span></div><div class="cardbody"><form id="distribution-editor-form" class="package-form" method="post" action="/collector-distributions"><div class="package-panel"><div class="package-panel-title">Distribution identity</div><div class="package-panel-subtitle">Name the package and pin it to an exact upstream Collector source.</div><div class="package-fields"><label class="package-field">Distribution name<input class="input" name="name" required maxlength="120" placeholder="Organization Collector"><span class="package-field-help">A clear name users will recognize when selecting a package.</span></label><label class="package-field">Collector version<input class="input" name="version" required maxlength="40" placeholder="0.140.0"><span class="package-field-help">The OpenTelemetry Collector version represented by this package.</span></label><label class="package-field full">Pinned source tag or commit<input class="input code" name="source_ref" required maxlength="160" placeholder="v0.140.0"><span class="package-field-help">Use an immutable release tag or commit so the source can be verified later.</span></label><label class="package-field">Published artifact URI <span class="tiny">Optional</span><input class="input code" name="artifact_uri" maxlength="500" placeholder="oci://registry.example/otelcol:0.140.0"><span class="package-field-help">Leave empty when FleetAMP will build and release the artifact.</span></label><label class="package-field">Artifact SHA-256 <span class="tiny">Optional</span><input class="input code" name="artifact_sha256" maxlength="64" placeholder="64 hexadecimal characters"><span class="package-field-help">Required when registering an already published artifact.</span></label></div></div><div class="package-panel"><div class="package-panel-title">Organization baseline Collector YAML</div><div class="package-panel-subtitle">Build the validated baseline section by section. FleetAMP combines these tabs into the immutable complete YAML. Use secret references—never raw credentials.</div><div id="distribution-validation-error" class="configerror" role="alert" aria-live="polite" hidden></div><textarea id="distribution-baseline" name="baseline_config" hidden></textarea><div class="section-tabs" role="tablist" aria-label="Collector configuration sections">{{range $index,$section := .Sections}}<button class="section-tab {{if eq $index 0}}active{{end}}" type="button" role="tab" aria-selected="{{if eq $index 0}}true{{else}}false{{end}}" data-section-tab="{{$section.Key}}">{{$section.Title}}</button>{{end}}<button class="section-tab" type="button" role="tab" aria-selected="false" data-section-tab="complete_preview">Complete YAML</button></div>{{range $index,$section := .Sections}}<section class="section-panel {{if eq $index 0}}active{{end}}" data-section-panel="{{$section.Key}}"><div class="section-heading"><div><strong>{{$section.Title}}</strong><div class="tiny">{{$section.Description}}</div></div><span class="badge ok">Editable</span></div><textarea class="section-editor" name="config_section_{{$section.Key}}" data-section-key="{{$section.Key}}" spellcheck="false" placeholder="Enter the {{$section.Title}} section value without the top-level key."></textarea></section>{{end}}<section class="section-panel" data-section-panel="complete_preview"><div class="section-heading"><div><strong>Complete Collector YAML</strong><div class="tiny">Generated and validated from every configuration tab.</div></div><span class="badge off">Generated preview</span></div><textarea id="distribution-preview" class="section-editor preview-editor" readonly spellcheck="false"></textarea></section><div class="detailactions"><button class="btn" id="distribution-preview-button" type="button">Validate and preview</button></div></div><div class="package-panel"><div class="package-panel-title">Included Collector components</div><div class="package-panel-subtitle">Enter one component name per line. These entries describe the capabilities compiled into the distribution.</div><div class="component-fields"><label class="package-field">Receivers<textarea class="input code" name="receivers" rows="5" placeholder="otlp&#10;prometheus"></textarea></label><label class="package-field">Processors<textarea class="input code" name="processors" rows="5" placeholder="memory_limiter&#10;batch"></textarea></label><label class="package-field">Exporters<textarea class="input code" name="exporters" rows="5" placeholder="otlphttp"></textarea></label><label class="package-field">Extensions<textarea class="input code" name="extensions" rows="4" placeholder="health_check"></textarea></label><label class="package-field">Connectors<textarea class="input code" name="connectors" rows="4" placeholder="spanmetrics"></textarea></label></div></div><div class="package-panel"><div class="package-panel-title">Configuration ownership</div><div class="package-panel-subtitle">Decide which baseline sections remain protected and which sections group owners may customize.</div><div class="section-options">{{range .Sections}}<div class="managed-field"><div><strong>{{.Title}}</strong><div class="tiny">{{.Description}}</div></div><div class="ownership-options"><label><input type="radio" name="section_{{.Key}}" value="locked" checked> Organization-managed</label><label><input type="radio" name="section_{{.Key}}" value="editable"> Group-editable</label></div></div>{{end}}</div></div><div class="notice"><strong>Immutable after saving.</strong> FleetAMP freezes the distribution contents and ownership boundary. Publish a new version to make corrections.</div><div class="package-actions"><button class="btn primary" type="submit">Validate and save distribution</button></div></form></div></section>{{end}}
<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Available Collector packages</div><div class="cardsub">Choose a released package, then create an authorized group configuration.</div></div></div><div class="cardbody"><div class="distribution-grid">{{range .Items}}<article class="proposal-card distribution-card"><div><strong>{{.Name}}</strong><div class="tiny">Collector {{.CollectorVersion}} · {{.SourceRef}}</div></div><div class="chips"><span class="chip">{{len .Components.Receivers}} receivers</span><span class="chip">{{len .Components.Processors}} processors</span><span class="chip">{{len .Components.Exporters}} exporters</span><span class="chip">{{len .Components.Extensions}} extensions</span><span class="chip">{{len .Components.Connectors}} connectors</span></div><div><strong>Organization-managed sections</strong><div class="chips">{{range .LockedSections}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Group-editable sections</strong><div class="chips">{{range .EditableSections}}<span class="chip">{{.}}</span>{{else}}<span class="tiny">No user-editable sections</span>{{end}}</div></div>{{if .BaselineConfig}}<div><strong>Admin baseline</strong><div class="tiny code">SHA-256: {{.BaselineHash}}</div></div>{{else}}<div class="configerror">Legacy entry: publish a new version with an Admin baseline before group configuration.</div>{{end}}{{if .ArtifactURI}}<div class="tiny code">{{.ArtifactURI}}</div>{{end}}{{if .ArtifactSHA256}}<div class="tiny code">SHA-256: {{.ArtifactSHA256}}</div>{{end}}<div class="tiny code">Specification: {{.SpecHash}}</div><div class="tiny">Published by {{.PublishedBy}} · {{.PublishedAt}}</div><a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a></article>{{else}}<div class="empty">No Collector packages are available.</div>{{end}}</div></div></section></div></main></div><script src="/assets/distribution-editor.js" defer></script></body></html>`

var distributionPage = template.Must(template.New("collector-distributions").Parse(strings.Replace(distributionHTML, `<a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a>`, `{{$release := index $.ReleaseByDistribution .ID}}{{if $release}}<div class="notice"><strong>Released package</strong><br><span class="tiny code">{{$release.ArtifactURI}}</span><br><span class="tiny code">SHA-256: {{$release.ArtifactSHA256}}</span></div>{{else if and .ArtifactURI .ArtifactSHA256}}<div class="notice"><strong>Legacy published package</strong><br><span class="tiny">Recorded before governed release evidence was introduced.</span></div>{{else}}<div class="tiny">Specification only · artifact release pending</div>{{end}}{{if or $release (and .ArtifactURI .ArtifactSHA256)}}<a class="btn primary" href="/collector-distributions/configure?id={{.ID}}">Configure for group</a>{{end}}`, 1)))

func registerDistributionRoutes(mux *http.ServeMux, store storage.DistributionStore, releaseStore storage.DistributionReleaseStore, lifecycle distributionLifecycleStores, validator *configs.Validator, auth *authManager) {
	mux.HandleFunc("GET /assets/distribution-editor.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(distributionEditorJS))
	})
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
				renderDistributionPage(w, r, store, releaseStore, lifecycle, isAdmin, "", "Invalid organization baseline: "+validation.Error)
				return
			}
			distribution, err := distributions.New(r.FormValue("name"), r.FormValue("version"), r.FormValue("source_ref"), r.FormValue("artifact_uri"), r.FormValue("artifact_sha256"), distributions.ComponentSet{Receivers: splitLines(r.FormValue("receivers")), Processors: splitLines(r.FormValue("processors")), Exporters: splitLines(r.FormValue("exporters")), Extensions: splitLines(r.FormValue("extensions")), Connectors: splitLines(r.FormValue("connectors"))}, baseline, locked, editable, currentUsername(auth, r), time.Now())
			if err == nil {
				err = store.Publish(r.Context(), distribution)
			}
			if err != nil {
				renderDistributionPage(w, r, store, releaseStore, lifecycle, isAdmin, "", err.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions?message="+url.QueryEscape("Collector package definition saved."), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderDistributionPage(w, r, store, releaseStore, lifecycle, isAdmin, r.URL.Query().Get("message"), "")
	})
}

func renderDistributionPage(w http.ResponseWriter, r *http.Request, store storage.DistributionStore, releaseStore storage.DistributionReleaseStore, lifecycle distributionLifecycleStores, isAdmin bool, message, pageError string) {
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
	stats, err := loadDistributionLifecycleStats(r, len(items), len(releases), lifecycle)
	if err != nil {
		internalServerError(w, err)
		return
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
	if err := distributionPage.Execute(w, distributionPageView{Page: "collector-distributions", IsAdmin: isAdmin, Items: items, Sections: configs.ConfigurationSectionDefinitions, ReleaseByDistribution: releaseByDistribution, Message: message, Error: pageError, Stats: stats}); err != nil {
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

func splitLines(value string) []string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}
