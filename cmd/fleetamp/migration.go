package main

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/migrations"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"gopkg.in/yaml.v3"
)

const migrationUploadLimit = 2 << 20
const migrationRequestLimit = migrationUploadLimit + (64 << 10)

type migrationComponentSummary struct {
	Section string
	Names   []string
}

type migrationStandardizationChange struct {
	Category string
	Summary  string
	Detail   string
}

type migrationHistoryStats struct {
	Total      int
	Groups     int
	Pattern    int
	Custom     int
	Last30Days int
}

type migrationView struct {
	Page             string
	Tab              string
	Groups           []*groups.Group
	Collectors       []*agents.ManagedAgent
	SelectedGroup    string
	SelectedAgent    string
	Source           string
	Name             string
	Version          string
	Content          string
	OriginalContent  string
	FileName         string
	Parsed           bool
	Standardized     bool
	ApplyBaseline    bool
	AdoptionReady    bool
	AdoptionChecks   []migrationAdoptionCheck
	PatternMatches   []migrationPatternMatch
	SelectedPattern  string
	PatternConfirmed bool
	PatternName      string
	PatternChanges   []migrationPatternAdoptionChange
	ProposedContent  string
	ProposedHash     string
	PatternAdopted   bool
	Validated        bool
	Validation       configs.ValidationResult
	PreviewReady     bool
	FinalHash        string
	History          []*migrations.Record
	HistoryStats     migrationHistoryStats
	SavedConfig      *configs.Configuration
	Components       []migrationComponentSummary
	Changes          []migrationStandardizationChange
	Warnings         []string
	Error            string
}

const migrationHTMLBase = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Configuration Migration · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.migration-grid{display:grid;grid-template-columns:minmax(340px,.8fr) minmax(0,1.2fr);gap:16px}.import-choice,.compare-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.dropzone{border:1px dashed #45648e;border-radius:10px;padding:15px;background:#0a1626}.yaml-input{width:100%;min-height:330px;resize:vertical;font-family:ui-monospace,SFMono-Regular,Menlo,monospace}.component-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.component-item,.change-item{border:1px solid var(--line);border-radius:9px;padding:12px;background:#0a1626}.migration-preview{white-space:pre;overflow:auto;max-height:620px}.compare-grid .migration-preview{min-height:420px;max-height:620px}.stage-note{padding:18px;border:1px dashed #304664;border-radius:10px;color:var(--muted)}.baseline-option{display:flex;align-items:flex-start;gap:10px;padding:14px;border:1px solid var(--line);border-radius:10px}.change-list{display:grid;gap:10px;margin-bottom:14px}.pattern-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.pattern-option{display:block;border:1px solid var(--line);border-radius:10px;padding:14px;background:#0a1626;cursor:pointer}.pattern-option:has(input:checked){border-color:#5b8cff;box-shadow:0 0 0 1px #5b8cff}.score{font-size:22px;font-weight:700}@media(max-width:1000px){.migration-grid,.import-choice,.component-list,.compare-grid,.pattern-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Migration</div><div class="pagetitle">Configuration Migration</div><div class="subtitle">Bring an existing OpenTelemetry Collector configuration into FleetAMP's governed lifecycle.</div></div><div class="topactions"><span class="badge ok">PR 4 · Pattern Matching</span></div></header><div class="content"><nav class="tabs" aria-label="Migration stages"><a class="tab {{if eq .Tab "import"}}active{{end}}" href="/migration?tab=import{{if .SelectedGroup}}&amp;group_id={{.SelectedGroup}}{{end}}">Import</a><a class="tab {{if eq .Tab "standardize"}}active{{end}}" href="/migration?tab=standardize">Standardization</a><a class="tab {{if eq .Tab "adoption"}}active{{end}}" href="/migration?tab=adoption">Collector Adoption</a><a class="tab {{if eq .Tab "pattern-match"}}active{{end}}" href="/migration?tab=pattern-match">Pattern Matching</a><a class="tab {{if eq .Tab "validate"}}active{{end}}" href="/migration?tab=validate">Validate <span class="soon">Next</span></a><a class="tab {{if eq .Tab "preview"}}active{{end}}" href="/migration?tab=preview">Preview <span class="soon">Planned</span></a><a class="tab {{if eq .Tab "history"}}active{{end}}" href="/migration?tab=history">History <span class="soon">Planned</span></a></nav>{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}{{if eq .Tab "import"}}<div class="migration-grid"><section class="card"><div class="cardhead"><div><div class="cardtitle">Import existing configuration</div><div class="cardsub">Select a managed Collector, paste YAML, or upload one .yaml/.yml file. Nothing is saved or deployed in this step.</div></div></div><div class="cardbody"><form method="post" action="/migration" enctype="multipart/form-data"><input type="hidden" name="tab" value="import"><div class="detailform"><label>Target group<select class="select" name="group_id" required><option value="">Select an accessible group</option>{{range .Groups}}<option value="{{.ID}}" {{if eq .ID $.SelectedGroup}}selected{{end}}>{{.Name}}</option>{{end}}</select></label><label>Configuration source<select class="select" name="source"><option value="existing-collector" {{if eq .Source "existing-collector"}}selected{{end}}>Existing Collector</option><option value="git-repository" {{if eq .Source "git-repository"}}selected{{end}}>Git repository</option><option value="kubernetes-configmap" {{if eq .Source "kubernetes-configmap"}}selected{{end}}>Kubernetes ConfigMap</option><option value="other" {{if eq .Source "other"}}selected{{end}}>Other</option></select></label><label>Managed Collector (optional)<select class="select" name="agent_uid"><option value="">Use pasted or uploaded YAML</option>{{range .Collectors}}<option value="{{.InstanceUID}}" {{if eq .InstanceUID $.SelectedAgent}}selected{{end}}>{{.Name}} · {{.Status}}</option>{{end}}</select></label><label>Configuration name<input class="input" name="name" value="{{.Name}}" required maxlength="160" placeholder="Existing production Collector"></label><label>Proposed version<input class="input" name="version" value="{{.Version}}" required maxlength="80" placeholder="import-1"></label></div><div class="import-choice" style="margin-top:14px"><label>Paste Collector YAML<textarea class="input yaml-input" name="yaml" spellcheck="false" placeholder="receivers:&#10;  otlp:&#10;    protocols:&#10;      grpc: {}">{{.Content}}</textarea></label><label class="dropzone">Upload YAML<input class="input" type="file" name="yaml_file" accept=".yaml,.yml,application/yaml,text/yaml,text/plain"><span class="tiny">Maximum request size: 2 MiB. Upload either a file or pasted YAML, not both.</span></label></div><div class="detailactions" style="margin-top:14px"><button class="btn primary" type="submit">Parse imported configuration</button></div></form></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Import result</div><div class="cardsub">Structural parsing only. Standardization, policy validation and saving follow in later stages.</div></div>{{if .Parsed}}<span class="badge ok">Parsed</span>{{else}}<span class="badge off">Waiting</span>{{end}}</div><div class="cardbody">{{if .Parsed}}<div class="component-list">{{range .Components}}<div class="component-item"><strong>{{.Section}}</strong><div class="chips">{{range .Names}}<span class="chip">{{.}}</span>{{else}}<span class="tiny">None</span>{{end}}</div></div>{{end}}</div>{{range .Warnings}}<div class="notice" style="margin-top:12px">{{.}}</div>{{end}}<div class="section-heading" style="margin-top:16px"><div><strong>Imported YAML</strong><div class="tiny">{{if .FileName}}{{.FileName}} · {{end}}Read-only parse preview</div></div></div><pre class="migration-preview">{{.Content}}</pre><form method="post" action="/migration?tab=standardize" enctype="multipart/form-data"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><textarea name="yaml" hidden>{{.Content}}</textarea><div class="detailactions"><button class="btn primary" type="submit">Continue to Standardization</button></div></form>{{else}}<div class="stage-note">Choose an accessible ownership group and import from a managed Collector, pasted YAML, or an uploaded file. FleetAMP will parse the document and inventory its components without changing it.</div>{{end}}</div></section></div>{{else if eq .Tab "standardize"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Standardize imported configuration</div><div class="cardsub">Create a deterministic FleetAMP layout and optionally add the safe Collector baseline.</div></div>{{if .Standardized}}<span class="badge ok">Standardized</span>{{else}}<span class="badge off">Waiting for import</span>{{end}}</div><div class="cardbody">{{if .Standardized}}<div class="change-list">{{range .Changes}}<div class="change-item"><span class="badge off">{{.Category}}</span> <strong>{{.Summary}}</strong><div class="tiny">{{.Detail}}</div></div>{{else}}<div class="notice">The imported configuration already follows the selected standard.</div>{{end}}</div><form method="post" action="/migration?tab=standardize" enctype="multipart/form-data"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><textarea name="yaml" hidden>{{.OriginalContent}}</textarea><label class="baseline-option"><input type="checkbox" name="apply_baseline" value="true" {{if .ApplyBaseline}}checked{{end}}><span><strong>Apply FleetAMP safe baseline</strong><span class="tiny" style="display:block">Add memory_limiter and batch only when missing, then reference them from each pipeline. Existing processors and their order are preserved.</span></span></label><div class="detailactions"><button class="btn" type="submit">Regenerate standardization</button></div></form>{{if .SelectedAgent}}<form method="post" action="/migration?tab=adoption" enctype="multipart/form-data"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><textarea name="original_yaml" hidden>{{.OriginalContent}}</textarea><textarea name="yaml" hidden>{{.Content}}</textarea><div class="detailactions"><button class="btn primary" type="submit">Review Collector Adoption</button></div></form>{{else}}<div class="notice">This configuration was pasted or uploaded and is not linked to a managed Collector. Import from a connected Collector to use the adoption readiness check.</div>{{end}}<div class="compare-grid"><div><div class="section-heading"><strong>Imported YAML</strong></div><pre class="migration-preview">{{.OriginalContent}}</pre></div><div><div class="section-heading"><strong>Standardized YAML</strong></div><pre class="migration-preview">{{.Content}}</pre></div></div>{{else}}<div class="stage-note">Import and parse a Collector configuration first. FleetAMP will then show every standardization change before any later validation or save step.</div>{{end}}</div></section>{{else if eq .Tab "adoption"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Collector Adoption</div><div class="cardsub">Verify identity, ownership and OpAMP readiness before the configuration enters governance.</div></div>{{if .AdoptionReady}}<span class="badge ok">Ready for validation</span>{{else}}<span class="badge warn">Action required</span>{{end}}</div><div class="cardbody">{{if .Standardized}}<div class="change-list">{{range .AdoptionChecks}}<div class="change-item"><span class="badge {{if eq .Status "Ready"}}ok{{else}}warn{{end}}">{{.Status}}</span> <strong>{{.Name}}</strong><div class="tiny">{{.Detail}}</div></div>{{end}}</div><div class="detailactions"><a class="btn" href="/agents/{{.SelectedAgent}}">Open Collector</a><a class="btn" href="/groups/{{.SelectedGroup}}">Open target group</a>{{if .AdoptionReady}}<form method="post" action="/migration?tab=pattern-match" enctype="multipart/form-data"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><textarea name="yaml" hidden>{{.Content}}</textarea><button class="btn primary" type="submit">Find matching Patterns</button></form>{{end}}</div><div class="section-heading"><div><strong>Candidate governed configuration</strong><div class="tiny">Read-only. No version, assignment or deployment is created in this step.</div></div></div><pre class="migration-preview">{{.Content}}</pre>{{else}}<div class="stage-note">Import from a connected Collector and complete Standardization first.</div>{{end}}</div></section>{{else if eq .Tab "pattern-match"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Pattern Matching</div><div class="cardsub">Compare the adopted configuration with enabled, administrator-governed Patterns.</div></div>{{if .PatternConfirmed}}<span class="badge ok">Selection confirmed</span>{{else}}<span class="badge off">Review required</span>{{end}}</div><div class="cardbody">{{if .Content}}<p class="tiny">Scores are explainable suggestions—not automatic conversions. Select a matched Pattern or explicitly keep this configuration custom.</p><form method="post" action="/migration?tab=pattern-match" enctype="multipart/form-data"><input type="hidden" name="action" value="confirm_pattern"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><textarea name="yaml" hidden>{{.Content}}</textarea><div class="pattern-grid">{{range .PatternMatches}}<label class="pattern-option"><div style="display:flex;justify-content:space-between;gap:12px"><span><input type="radio" name="pattern_id" value="{{.ID}}" {{if eq .ID $.SelectedPattern}}checked{{end}}> <strong>{{.Name}}</strong></span>{{if .Recommended}}<span class="badge ok">Recommended</span>{{end}}</div><div class="score">{{.Score}}%</div><div class="tiny">{{.Confidence}} confidence · {{.Platform}}</div><p>{{.Description}}</p><div class="chips">{{range .Signals}}<span class="chip">{{.}}</span>{{end}}</div><ul class="tiny">{{range .Evidence}}<li>{{.}}</li>{{end}}</ul></label>{{else}}<div class="notice">No enabled Pattern shares a receiver with this configuration. Keep it custom or ask an administrator to publish an appropriate Pattern.</div>{{end}}<label class="pattern-option"><input type="radio" name="pattern_id" value="custom" {{if eq .SelectedPattern "custom"}}checked{{end}}> <strong>Keep as custom configuration</strong><p class="tiny">Preserve the standardized configuration without claiming conformance to an approved Pattern.</p></label></div><div class="detailactions"><button class="btn" type="submit">Confirm Pattern selection</button>{{if .PatternConfirmed}}<span class="btn primary" aria-disabled="true">Continue to Validate · Next PR</span>{{end}}</div></form><div class="section-heading"><div><strong>Analyzed configuration</strong><div class="tiny">Read-only; matching does not modify YAML.</div></div></div><pre class="migration-preview">{{.Content}}</pre>{{else}}<div class="stage-note">Complete Collector Adoption before Pattern matching.</div>{{end}}</div></section>{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div><div class="cardsub">This stage will be implemented after the import boundary is merged.</div></div><span class="badge warn">Upcoming PR</span></div><div class="cardbody"><div class="stage-note">The Migration workspace is intentionally staged: Import → Standardize → Collector Adoption → Pattern Match → Validate → Preview → Save version. No stage bypasses group ownership, locked sections, approval or atomic deployment.</div></div></section>{{end}}</div></main></div></body></html>`

var migrationHTMLWithAdoption = strings.NewReplacer(
	`<input type="hidden" name="action" value="confirm_adoption">`, ``,
	`<textarea name="yaml" hidden>{{.Content}}</textarea><div class="detailactions"><a class="btn" href="/migration?tab=pattern-match">Back to matching</a>`, `<textarea name="yaml" hidden>{{.Content}}</textarea><input type="hidden" name="proposal_hash" value="{{.ProposedHash}}"><div class="detailactions"><button class="btn" type="submit" formaction="/migration?tab=pattern-match" name="action" value="confirm_pattern">Back to matching</button>`,
	`<button class="btn primary" type="submit">{{if eq .SelectedPattern "custom"}}`, `<button class="btn primary" type="submit" name="action" value="confirm_adoption">{{if eq .SelectedPattern "custom"}}`,
).Replace(strings.NewReplacer(
	"PR 4 · Pattern Matching", "PR 5 · Adopt / Upgrade Pattern",
	`<nav class="tabs" aria-label="Migration stages"><a class="tab {{if eq .Tab "import"}}active{{end}}"`, `<nav class="tabs" aria-label="Migration stages"><a class="tab" href="/migration/fleet-adoption">Fleet Adoption</a><a class="tab {{if eq .Tab "import"}}active{{end}}"`,
	`Pattern Matching</a><a class="tab {{if eq .Tab "validate"}}active{{end}}"`, `Pattern Matching</a><a class="tab {{if eq .Tab "pattern-adopt"}}active{{end}}" href="/migration?tab=pattern-adopt">Adopt / Upgrade</a><a class="tab {{if eq .Tab "validate"}}active{{end}}"`,
	`<span class="btn primary" aria-disabled="true">Continue to Validate · Next PR</span>`, `<button class="btn primary" type="submit" formaction="/migration?tab=pattern-adopt" name="action" value="preview_adoption">Compare Pattern changes</button>`,
	`{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`, `{{else if eq .Tab "pattern-adopt"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Adopt / Upgrade to Pattern</div><div class="cardsub">Review the complete configuration before explicitly adopting the current governed Pattern.</div></div>{{if .PatternAdopted}}<span class="badge ok">Adoption confirmed</span>{{else}}<span class="badge off">Preview only</span>{{end}}</div><div class="cardbody">{{if .ProposedContent}}<div class="notice">Pattern: <strong>{{.PatternName}}</strong> <span class="code">{{.SelectedPattern}}</span>. Only the matched receiver is replaced; all other Collector sections remain unchanged.</div><div class="change-list" style="margin-top:12px">{{range .PatternChanges}}<div class="change-item"><span class="badge off">{{.Category}}</span> <strong>{{.Summary}}</strong><div class="tiny">{{.Detail}}</div></div>{{else}}<div class="notice">This configuration already conforms to the selected Pattern.</div>{{end}}</div><div class="compare-grid"><div><div class="section-heading"><strong>Current standardized YAML</strong></div><pre class="migration-preview">{{.Content}}</pre></div><div><div class="section-heading"><strong>Proposed governed YAML</strong></div><pre class="migration-preview">{{.ProposedContent}}</pre></div></div><form method="post" action="/migration?tab=pattern-adopt" enctype="multipart/form-data"><input type="hidden" name="action" value="confirm_adoption"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><input type="hidden" name="pattern_id" value="{{.SelectedPattern}}"><textarea name="yaml" hidden>{{.Content}}</textarea><div class="detailactions"><a class="btn" href="/migration?tab=pattern-match">Back to matching</a><button class="btn primary" type="submit">{{if eq .SelectedPattern "custom"}}Confirm custom configuration{{else}}Adopt Pattern changes{{end}}</button>{{if .PatternAdopted}}<span class="btn primary" aria-disabled="true">Continue to Validate · Next PR</span>{{end}}</div></form>{{else}}<div class="stage-note">Confirm a Pattern match or choose custom configuration before this stage.</div>{{end}}</div></section>{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`,
	"Pattern Match → Validate", "Pattern Match → Adopt / Upgrade → Validate",
).Replace(migrationHTMLBase))

var migrationHTMLWithPreview = strings.NewReplacer(
	`Validate <span class="soon">Next</span>`, `Validate`,
	`Preview <span class="soon">Planned</span>`, `Preview`,
	`<textarea name="yaml" hidden>{{.Content}}</textarea><input type="hidden" name="proposal_hash" value="{{.ProposedHash}}">`, `<textarea name="yaml" hidden>{{.Content}}</textarea><textarea name="proposed_yaml" hidden>{{.ProposedContent}}</textarea><input type="hidden" name="proposal_hash" value="{{.ProposedHash}}">`,
	`<span class="btn primary" aria-disabled="true">Continue to Validate · Next PR</span>`, `<button class="btn primary" type="submit" formaction="/migration?tab=validate" name="action" value="validate">Continue to Validate</button>`,
	`{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`, `{{else if eq .Tab "validate"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Validate migration candidate</div><div class="cardsub">Check YAML, pipeline references, secret references, and the configured Collector distribution before saving.</div></div>{{if .Validation.Valid}}<span class="badge ok">Valid</span>{{else if .Validated}}<span class="badge warn">Blocked</span>{{else}}<span class="badge off">Waiting</span>{{end}}</div><div class="cardbody">{{if .Validated}}<div class="change-list"><div class="change-item"><span class="badge {{if .Validation.YAMLValid}}ok{{else}}warn{{end}}">{{if .Validation.YAMLValid}}Passed{{else}}Failed{{end}}</span> <strong>YAML and pipeline structure</strong><div class="tiny">Component references and service pipelines must form a valid Collector document.</div></div><div class="change-item"><span class="badge {{if .Validation.CollectorValidated}}ok{{else}}off{{end}}">{{if .Validation.CollectorValidated}}Passed{{else if .Validation.CollectorSkipped}}Skipped{{else}}Not run{{end}}</span> <strong>Collector distribution</strong><div class="tiny">{{if .Validation.CollectorSkipped}}Configure FLEETAMP_OTELCOL_BINARY to enable distribution-specific validation.{{else}}Validated with the configured OpenTelemetry Collector binary.{{end}}</div></div></div>{{range .Validation.Warnings}}<div class="notice">{{.}}</div>{{end}}{{if .Validation.Error}}<div class="configerror" role="alert">{{.Validation.Error}}</div>{{end}}<div class="section-heading"><div><strong>Final candidate YAML</strong><div class="tiny">Read-only. Validation does not create a version or deployment.</div></div></div><pre class="migration-preview">{{.ProposedContent}}</pre>{{if .Validation.Valid}}<form method="post" action="/migration?tab=preview" enctype="multipart/form-data"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><input type="hidden" name="pattern_id" value="{{.SelectedPattern}}"><input type="hidden" name="proposal_hash" value="{{.FinalHash}}"><textarea name="proposed_yaml" hidden>{{.ProposedContent}}</textarea><div class="detailactions"><button class="btn primary" type="submit">Continue to final Preview</button></div></form>{{end}}{{else}}<div class="stage-note">Complete and confirm Pattern adoption before validation. Direct navigation cannot bypass the earlier migration stages.</div>{{end}}</div></section>{{else if eq .Tab "preview"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Final preview before saving</div><div class="cardsub">Review the exact metadata and immutable YAML candidate that a future save action will use.</div></div>{{if .PreviewReady}}<span class="badge ok">Ready to save</span>{{else}}<span class="badge off">Waiting for validation</span>{{end}}</div><div class="cardbody">{{if .PreviewReady}}<div class="component-list"><div class="component-item"><strong>Target group</strong><div class="tiny code">{{.SelectedGroup}}</div></div><div class="component-item"><strong>Name and version</strong><div class="tiny">{{.Name}} · <span class="code">{{.Version}}</span></div></div><div class="component-item"><strong>Source</strong><div class="tiny">{{.Source}}{{if .SelectedAgent}} · {{.SelectedAgent}}{{end}}</div></div><div class="component-item"><strong>Pattern decision</strong><div class="tiny">{{if eq .SelectedPattern "custom"}}Custom configuration{{else}}{{.SelectedPattern}}{{end}}</div></div></div><div class="section-heading"><div><strong>Content fingerprint</strong><div class="tiny code">SHA-256 · {{.FinalHash}}</div></div></div><pre class="migration-preview">{{.ProposedContent}}</pre><div class="notice">Saving remains disabled in this change. The next stage will create the version through the governed configuration workflow without deploying it automatically.</div><div class="detailactions"><span class="btn primary" aria-disabled="true">Save version · Next stage</span></div>{{else}}<div class="stage-note">A successful server-side validation is required immediately before Preview. Return to Pattern adoption and continue through Validate.</div>{{end}}</div></section>{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`,
).Replace(migrationHTMLWithAdoption)

var migrationHTMLWithHistory = strings.NewReplacer(
	`History <span class="soon">Planned</span>`, `History`,
	`<div class="notice">Saving remains disabled in this change. The next stage will create the version through the governed configuration workflow without deploying it automatically.</div><div class="detailactions"><span class="btn primary" aria-disabled="true">Save version · Next stage</span></div>`, `<div class="notice">Saving creates an immutable group configuration version only. It does not assign, submit for approval, or deploy the configuration.</div><form method="post" action="/migration?tab=history" enctype="multipart/form-data"><input type="hidden" name="action" value="save_migration"><input type="hidden" name="group_id" value="{{.SelectedGroup}}"><input type="hidden" name="source" value="{{.Source}}"><input type="hidden" name="agent_uid" value="{{.SelectedAgent}}"><input type="hidden" name="name" value="{{.Name}}"><input type="hidden" name="version" value="{{.Version}}"><input type="hidden" name="pattern_id" value="{{.SelectedPattern}}"><input type="hidden" name="proposal_hash" value="{{.FinalHash}}"><textarea name="proposed_yaml" hidden>{{.ProposedContent}}</textarea><div class="detailactions"><button class="btn primary" type="submit">Save immutable version</button></div></form>`,
	`{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`, `{{else if eq .Tab "history"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Migration History</div><div class="cardsub">Append-only provenance for configuration versions created through the Migration workspace.</div></div><span class="badge ok">Immutable records</span></div><div class="cardbody">{{if .SavedConfig}}<div class="notice">✓ Saved <strong>{{.SavedConfig.Name}}</strong> version <span class="code">{{.SavedConfig.Version}}</span>. No assignment, approval request, or deployment was created.</div><div class="detailactions"><a class="btn primary" href="/groups/{{.SavedConfig.GroupID}}?configuration_saved={{.SavedConfig.ID}}">Open saved group version</a></div>{{end}}{{if .History}}<div style="overflow:auto"><table><thead><tr><th>Saved</th><th>Group</th><th>Configuration</th><th>Source</th><th>Pattern</th><th>Created by</th><th>Hash</th></tr></thead><tbody>{{range .History}}<tr><td>{{.CreatedAt}}</td><td><a href="/groups/{{.GroupID}}"><strong>{{.GroupName}}</strong></a></td><td>{{.Name}} <span class="code">{{.Version}}</span><div class="tiny code">{{.ConfigurationID}}</div></td><td>{{.Source}}{{if .AgentUID}}<div class="tiny code">{{.AgentUID}}</div>{{end}}</td><td>{{if eq .PatternID "custom"}}Custom{{else}}<span class="code">{{.PatternID}}</span>{{end}}</td><td>{{.CreatedBy}}</td><td><span class="code">{{.ContentHash}}</span></td></tr>{{end}}</tbody></table></div>{{else}}<div class="stage-note">No migration results have been saved for your accessible groups.</div>{{end}}</div></section>{{else}}<section class="card"><div class="cardhead"><div><div class="cardtitle">{{.Tab}}</div>`,
).Replace(migrationHTMLWithPreview)

var migrationHTML = strings.NewReplacer(
	`<div class="cardbody">{{if .SavedConfig}}<div class="notice">✓ Saved`, `<div class="cardbody"><div class="component-list" style="margin-bottom:16px"><div class="component-item"><strong>{{.HistoryStats.Total}}</strong><div class="tiny">Saved migrations</div></div><div class="component-item"><strong>{{.HistoryStats.Groups}}</strong><div class="tiny">Groups covered</div></div><div class="component-item"><strong>{{.HistoryStats.Pattern}}</strong><div class="tiny">Pattern governed</div></div><div class="component-item"><strong>{{.HistoryStats.Custom}}</strong><div class="tiny">Custom configurations</div></div><div class="component-item"><strong>{{.HistoryStats.Last30Days}}</strong><div class="tiny">Saved in last 30 days</div></div></div>{{if .SavedConfig}}<div class="notice">✓ Saved`,
	`<div class="detailactions"><a class="btn primary" href="/groups/{{.SavedConfig.GroupID}}?configuration_saved={{.SavedConfig.ID}}">Open saved group version</a></div>`, `<div class="detailactions"><a class="btn" href="/groups/{{.SavedConfig.GroupID}}?configuration_saved={{.SavedConfig.ID}}">Open saved group version</a><a class="btn primary" href="/groups/{{.SavedConfig.GroupID}}?configuration_id={{.SavedConfig.ID}}">Review &amp; request approval</a></div>`,
	`<th>Created by</th><th>Hash</th></tr>`, `<th>Created by</th><th>Hash</th><th>Next step</th></tr>`,
	`<td><span class="code">{{.ContentHash}}</span></td></tr>`, `<td><span class="code">{{.ContentHash}}</span></td><td><a class="btn" href="/groups/{{.GroupID}}?configuration_id={{.ConfigurationID}}">Request approval</a></td></tr>`,
).Replace(migrationHTMLWithHistory)

var migrationPage = template.Must(template.New("migration").Parse(migrationHTML))

func registerMigrationRoutes(mux *http.ServeMux, groupStore storage.GroupStore, patternStore storage.DestinationProfileStore, configStore storage.ConfigurationStore, migrationStore storage.MigrationStore, agentStore *memory.AgentStore, adapter *fleetopamp.Adapter, validator *configs.Validator, policyStore storage.SectionPolicyStore, auth *authManager) {
	registerFleetAdoptionRoute(mux, groupStore, patternStore, agentStore, adapter, auth)
	mux.HandleFunc("/migration", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/migration" {
			http.NotFound(w, r)
			return
		}
		tab := strings.TrimSpace(r.URL.Query().Get("tab"))
		if tab == "" {
			tab = "import"
		}
		switch tab {
		case "import", "standardize", "adoption", "pattern-match", "pattern-adopt", "validate", "preview", "history":
		default:
			http.Error(w, "unknown migration tab", http.StatusBadRequest)
			return
		}
		allGroups, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		username, principalRole := currentUsername(auth, r), currentRole(auth, r)
		visibleGroups := groupsVisibleToUser(r.Context(), auth, username, principalRole, allGroups)
		allCollectors, err := agentStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		visibleCollectors := migrationVisibleCollectors(allCollectors, visibleGroups, principalRole)
		view := migrationView{Page: "migration", Tab: tab, Groups: visibleGroups, Collectors: visibleCollectors, SelectedGroup: strings.TrimSpace(r.URL.Query().Get("group_id")), SelectedAgent: strings.TrimSpace(r.URL.Query().Get("agent_uid")), Source: "existing-collector"}
		if r.Method == http.MethodGet {
			if tab == "history" {
				view.History, err = visibleMigrationHistory(r, migrationStore, visibleGroups)
				if err != nil {
					internalServerError(w, err)
					return
				}
				view.HistoryStats = buildMigrationHistoryStats(view.History, time.Now().UTC())
				if savedID := strings.TrimSpace(r.URL.Query().Get("saved")); savedID != "" {
					saved, getErr := configStore.Get(r.Context(), savedID)
					if getErr == nil && configurationVisibleToGroups(saved, visibleGroups) {
						view.SavedConfig = saved
					}
				}
			}
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, migrationRequestLimit)
		if err := r.ParseMultipartForm(migrationRequestLimit); err != nil {
			view.Error = "Import exceeds 2 MiB or the form is invalid."
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		view.SelectedGroup = strings.TrimSpace(r.FormValue("group_id"))
		view.SelectedAgent = strings.TrimSpace(r.FormValue("agent_uid"))
		view.Source = strings.TrimSpace(r.FormValue("source"))
		view.Name = strings.TrimSpace(r.FormValue("name"))
		view.Version = strings.TrimSpace(r.FormValue("version"))
		group, err := groupStore.Get(r.Context(), view.SelectedGroup)
		if err != nil || !groupVisibleIn(group, visibleGroups) {
			view.Error = "Select an accessible target group."
			w.WriteHeader(http.StatusForbidden)
			_ = migrationPage.Execute(w, view)
			return
		}
		if !group.Enabled {
			view.Error = "The target group must be enabled."
			w.WriteHeader(http.StatusConflict)
			_ = migrationPage.Execute(w, view)
			return
		}
		if view.Name == "" || view.Version == "" {
			view.Error = "Configuration name and proposed version are required."
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		if tab == "history" {
			if r.FormValue("action") != "save_migration" {
				http.Error(w, "unknown migration history action", http.StatusBadRequest)
				return
			}
			if !canManageGroup(auth, r, group) {
				http.Error(w, "only an Admin or assigned Group Owner can save a group configuration version", http.StatusForbidden)
				return
			}
			view.SelectedPattern = strings.TrimSpace(r.FormValue("pattern_id"))
			view.ProposedContent = strings.TrimSpace(r.FormValue("proposed_yaml"))
			if view.Source == "" {
				view.Source = "other"
			}
			view.FinalHash = migrationProposalHash(view.ProposedContent, view.SelectedPattern)
			if view.SelectedPattern == "" || view.ProposedContent == "" || strings.TrimSpace(r.FormValue("proposal_hash")) != view.FinalHash {
				view.Error = "The final Preview changed or is incomplete. Validate and review the migration again before saving."
				w.WriteHeader(http.StatusConflict)
				_ = migrationPage.Execute(w, view)
				return
			}
			validation := validateMigrationCandidate(r, validator, policyStore, principalRole, view.SelectedGroup, view.SelectedAgent, view.ProposedContent, visibleCollectors, adapter)
			if !validation.Valid {
				view.Error = "The migration candidate no longer passes validation: " + strings.TrimSpace(validation.Error)
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			if view.SelectedPattern != "custom" {
				patterns, patternErr := migrationCatalogPatterns(r.Context(), patternStore)
				if patternErr != nil {
					internalServerError(w, patternErr)
					return
				}
				if enabledPatternByID(patterns, view.SelectedPattern) == nil {
					view.Error = "The selected Pattern is no longer enabled. Return to Pattern Matching before saving."
					w.WriteHeader(http.StatusConflict)
					_ = migrationPage.Execute(w, view)
					return
				}
			}
			existing, listErr := configStore.List(r.Context())
			if listErr != nil {
				internalServerError(w, listErr)
				return
			}
			for _, configuration := range configurationsForGroup(existing, group.ID) {
				if configuration.Name == view.Name && configuration.Version == view.Version {
					view.Error = "Configuration name and version already exist in this group. Return to Import and choose a new version."
					w.WriteHeader(http.StatusConflict)
					_ = migrationPage.Execute(w, view)
					return
				}
			}
			configuration := configs.NewGroupConfiguration(group.ID, view.Name, view.Version, view.ProposedContent, "text/yaml")
			record := &migrations.Record{
				ID: configuration.ID, ConfigurationID: configuration.ID, GroupID: group.ID, GroupName: group.Name,
				Name: configuration.Name, Version: configuration.Version, Source: view.Source, AgentUID: view.SelectedAgent,
				PatternID: view.SelectedPattern, ContentHash: configuration.Hash, CreatedBy: username, CreatedAt: configuration.CreatedAt,
			}
			if err := migrationStore.Save(r.Context(), configuration, record); err != nil {
				internalServerError(w, err)
				return
			}
			http.Redirect(w, r, "/migration?tab=history&saved="+configuration.ID, http.StatusSeeOther)
			return
		}
		if tab == "standardize" {
			view.OriginalContent = strings.TrimSpace(r.FormValue("yaml"))
			if view.OriginalContent == "" {
				view.Error = "Import and parse a Collector YAML configuration before standardization."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			if _, _, err := inspectImportedConfiguration(view.OriginalContent); err != nil {
				view.Error = err.Error()
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.ApplyBaseline = r.FormValue("apply_baseline") == "true"
			standardized, changes, err := standardizeImportedConfiguration(view.OriginalContent, view.ApplyBaseline)
			if err != nil {
				view.Error = err.Error()
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.Content, view.Changes, view.Standardized = standardized, changes, true
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if tab == "adoption" {
			view.OriginalContent = strings.TrimSpace(r.FormValue("original_yaml"))
			view.Content = strings.TrimSpace(r.FormValue("yaml"))
			if view.Content == "" || view.SelectedAgent == "" {
				view.Error = "Select and import a connected Collector before reviewing adoption."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			if _, _, err := inspectImportedConfiguration(view.Content); err != nil {
				view.Error = err.Error()
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			agent := migrationCollectorByID(visibleCollectors, view.SelectedAgent)
			if agent == nil {
				view.Error = "The selected Collector is outside your accessible groups."
				w.WriteHeader(http.StatusForbidden)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.AdoptionChecks, view.AdoptionReady = assessCollectorAdoption(agent, group, adapter.EffectiveConfig(agent.InstanceUID))
			view.Standardized = true
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if tab == "pattern-match" {
			view.Content = strings.TrimSpace(r.FormValue("yaml"))
			if view.Content == "" || view.SelectedAgent == "" {
				view.Error = "Complete Collector Adoption before matching a Pattern."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			agent := migrationCollectorByID(visibleCollectors, view.SelectedAgent)
			if agent == nil {
				view.Error = "The selected Collector is outside your accessible groups."
				w.WriteHeader(http.StatusForbidden)
				_ = migrationPage.Execute(w, view)
				return
			}
			checks, ready := assessCollectorAdoption(agent, group, adapter.EffectiveConfig(agent.InstanceUID))
			view.AdoptionChecks, view.AdoptionReady = checks, ready
			if !ready {
				view.Error = "Resolve the blocking Collector Adoption checks before Pattern matching."
				w.WriteHeader(http.StatusConflict)
				_ = migrationPage.Execute(w, view)
				return
			}
			patterns, err := migrationCatalogPatterns(r.Context(), patternStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			view.PatternMatches, err = matchCollectorPatterns(view.Content, patterns)
			if err != nil {
				view.Error = err.Error()
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.SelectedPattern = strings.TrimSpace(r.FormValue("pattern_id"))
			if r.FormValue("action") == "confirm_pattern" {
				if !validPatternSelection(view.SelectedPattern, view.PatternMatches) {
					view.Error = "Select a matched Pattern or explicitly keep this configuration custom."
					w.WriteHeader(http.StatusUnprocessableEntity)
					_ = migrationPage.Execute(w, view)
					return
				}
				view.PatternConfirmed = true
			}
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if tab == "pattern-adopt" {
			view.Content = strings.TrimSpace(r.FormValue("yaml"))
			view.SelectedPattern = strings.TrimSpace(r.FormValue("pattern_id"))
			if view.Content == "" || view.SelectedAgent == "" || view.SelectedPattern == "" {
				view.Error = "Confirm a Pattern match or choose custom configuration before adoption."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			agent := migrationCollectorByID(visibleCollectors, view.SelectedAgent)
			if agent == nil {
				view.Error = "The selected Collector is outside your accessible groups."
				w.WriteHeader(http.StatusForbidden)
				_ = migrationPage.Execute(w, view)
				return
			}
			checks, ready := assessCollectorAdoption(agent, group, adapter.EffectiveConfig(agent.InstanceUID))
			view.AdoptionChecks, view.AdoptionReady = checks, ready
			if !ready {
				view.Error = "Resolve the blocking Collector Adoption checks before adopting a Pattern."
				w.WriteHeader(http.StatusConflict)
				_ = migrationPage.Execute(w, view)
				return
			}
			patterns, err := migrationCatalogPatterns(r.Context(), patternStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			matches, err := matchCollectorPatterns(view.Content, patterns)
			if err != nil {
				view.Error = err.Error()
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			if !validPatternSelection(view.SelectedPattern, matches) {
				view.Error = "The selected Pattern is disabled, unavailable, or no longer matches this configuration."
				w.WriteHeader(http.StatusConflict)
				_ = migrationPage.Execute(w, view)
				return
			}
			if view.SelectedPattern == "custom" {
				view.PatternName = "Custom configuration"
				view.ProposedContent = view.Content
				view.PatternChanges = []migrationPatternAdoptionChange{{Category: "Decision", Summary: "Keep the configuration custom", Detail: "No Pattern-owned receiver settings are applied."}}
			} else {
				pattern := enabledPatternByID(patterns, view.SelectedPattern)
				if pattern == nil {
					view.Error = "The selected Pattern is no longer enabled."
					w.WriteHeader(http.StatusConflict)
					_ = migrationPage.Execute(w, view)
					return
				}
				view.PatternName = pattern.Name
				view.ProposedContent, view.PatternChanges, err = adoptCollectorPattern(view.Content, pattern)
				if err != nil {
					view.Error = err.Error()
					w.WriteHeader(http.StatusUnprocessableEntity)
					_ = migrationPage.Execute(w, view)
					return
				}
			}
			view.ProposedHash = migrationProposalHash(view.ProposedContent, view.SelectedPattern)
			if r.FormValue("action") == "confirm_adoption" {
				if submitted := strings.TrimSpace(r.FormValue("proposal_hash")); submitted == "" || submitted != view.ProposedHash {
					view.Error = "The Pattern proposal changed or was not previewed. Review the current proposal before confirming adoption."
					w.WriteHeader(http.StatusConflict)
					_ = migrationPage.Execute(w, view)
					return
				}
				view.PatternAdopted = true
			}
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if tab == "validate" {
			view.SelectedPattern = strings.TrimSpace(r.FormValue("pattern_id"))
			view.ProposedContent = strings.TrimSpace(r.FormValue("proposed_yaml"))
			view.FinalHash = migrationProposalHash(view.ProposedContent, view.SelectedPattern)
			view.Validated = true
			if r.FormValue("action") != "validate" || view.SelectedPattern == "" || view.ProposedContent == "" || strings.TrimSpace(r.FormValue("proposal_hash")) != view.FinalHash {
				view.Error = "Complete and confirm Pattern adoption before validation."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.Validation = validateMigrationCandidate(r, validator, policyStore, principalRole, view.SelectedGroup, view.SelectedAgent, view.ProposedContent, visibleCollectors, adapter)
			if !view.Validation.Valid {
				w.WriteHeader(http.StatusUnprocessableEntity)
			}
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		if tab == "preview" {
			view.SelectedPattern = strings.TrimSpace(r.FormValue("pattern_id"))
			view.ProposedContent = strings.TrimSpace(r.FormValue("proposed_yaml"))
			view.FinalHash = migrationProposalHash(view.ProposedContent, view.SelectedPattern)
			if view.SelectedPattern == "" || view.ProposedContent == "" || strings.TrimSpace(r.FormValue("proposal_hash")) != view.FinalHash {
				view.Error = "A validated migration candidate is required before Preview."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.Validation = validateMigrationCandidate(r, validator, policyStore, principalRole, view.SelectedGroup, view.SelectedAgent, view.ProposedContent, visibleCollectors, adapter)
			view.Validated = true
			if !view.Validation.Valid {
				view.Error = "The candidate no longer passes validation. Return to Validate and resolve the reported error."
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = migrationPage.Execute(w, view)
				return
			}
			view.PreviewReady = true
			if err := migrationPage.Execute(w, view); err != nil {
				internalServerError(w, err)
			}
			return
		}
		pasted := strings.TrimSpace(r.FormValue("yaml"))
		uploaded, fileName, uploadErr := readMigrationUpload(r)
		if uploadErr != nil {
			view.Error = uploadErr.Error()
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		effective := ""
		if view.SelectedAgent != "" {
			agent := migrationCollectorByID(visibleCollectors, view.SelectedAgent)
			if agent == nil {
				view.Error = "The selected Collector is outside your accessible groups."
				w.WriteHeader(http.StatusForbidden)
				_ = migrationPage.Execute(w, view)
				return
			}
			effective = strings.TrimSpace(adapter.EffectiveConfig(agent.InstanceUID))
			if effective == "" {
				view.Error = "The selected Collector has not reported an effective configuration."
				w.WriteHeader(http.StatusConflict)
				_ = migrationPage.Execute(w, view)
				return
			}
		}
		provided := 0
		for _, candidate := range []string{pasted, uploaded, effective} {
			if candidate != "" {
				provided++
			}
		}
		if provided > 1 {
			view.Content = pasted
			view.Error = "Choose one source: a connected Collector, pasted YAML, or an uploaded file."
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		view.Content, view.FileName = pasted, fileName
		if view.Content == "" {
			view.Content = uploaded
		}
		if view.Content == "" {
			view.Content = effective
		}
		if strings.TrimSpace(view.Content) == "" {
			view.Error = "Paste or upload an OpenTelemetry Collector YAML configuration."
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		components, warnings, parseErr := inspectImportedConfiguration(view.Content)
		if parseErr != nil {
			view.Error = parseErr.Error()
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = migrationPage.Execute(w, view)
			return
		}
		view.Parsed, view.Components, view.Warnings = true, components, warnings
		if err := migrationPage.Execute(w, view); err != nil {
			internalServerError(w, err)
		}
	})
}

func visibleMigrationHistory(r *http.Request, store storage.MigrationStore, visibleGroups []*groups.Group) ([]*migrations.Record, error) {
	records, err := store.List(r.Context())
	if err != nil {
		return nil, err
	}
	visible := make(map[string]bool, len(visibleGroups))
	for _, group := range visibleGroups {
		visible[group.ID] = true
	}
	result := make([]*migrations.Record, 0, len(records))
	for _, record := range records {
		if visible[record.GroupID] {
			result = append(result, record)
		}
	}
	return result, nil
}

func configurationVisibleToGroups(configuration *configs.Configuration, visibleGroups []*groups.Group) bool {
	if configuration == nil {
		return false
	}
	for _, group := range visibleGroups {
		if group.ID == configuration.GroupID {
			return true
		}
	}
	return false
}

func buildMigrationHistoryStats(records []*migrations.Record, now time.Time) migrationHistoryStats {
	stats := migrationHistoryStats{Total: len(records)}
	groupsSeen := map[string]bool{}
	cutoff := now.UTC().Add(-30 * 24 * time.Hour)
	for _, record := range records {
		if record == nil {
			continue
		}
		groupsSeen[record.GroupID] = true
		if record.PatternID == "custom" {
			stats.Custom++
		} else {
			stats.Pattern++
		}
		if !record.CreatedAt.Before(cutoff) {
			stats.Last30Days++
		}
	}
	stats.Groups = len(groupsSeen)
	return stats
}

func validateMigrationCandidate(r *http.Request, validator *configs.Validator, policyStore storage.SectionPolicyStore, principalRole role, groupID, agentID, content string, visibleCollectors []*agents.ManagedAgent, adapter *fleetopamp.Adapter) configs.ValidationResult {
	resolved, err := configurationContentForValidation(r.Context(), groupID, content)
	if err != nil {
		return configs.ValidationResult{Error: err.Error()}
	}
	result := validator.Validate(r.Context(), resolved)
	if !result.Valid {
		return result
	}
	agent := migrationCollectorByID(visibleCollectors, agentID)
	if agent == nil {
		result.Valid = false
		result.Error = "the migration Collector is no longer accessible"
		return result
	}
	baseline := strings.TrimSpace(adapter.EffectiveConfig(agent.InstanceUID))
	if baseline == "" {
		result.Valid = false
		result.Error = "the migration Collector has no effective configuration for policy comparison"
		return result
	}
	policies, err := policyStore.List(r.Context())
	if err != nil {
		result.Valid = false
		result.Error = "load configuration section policies: " + err.Error()
		return result
	}
	if err := enforceSectionPolicies(baseline, content, policies, principalRole); err != nil {
		result.Valid = false
		result.Error = err.Error()
	}
	return result
}

func groupVisibleIn(wanted *groups.Group, visible []*groups.Group) bool {
	if wanted == nil {
		return false
	}
	for _, group := range visible {
		if group.ID == wanted.ID {
			return true
		}
	}
	return false
}

func readMigrationUpload(r *http.Request) (string, string, error) {
	file, header, err := r.FormFile("yaml_file")
	if errors.Is(err, http.ErrMissingFile) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read uploaded YAML: %w", err)
	}
	defer file.Close()
	extension := strings.ToLower(filepath.Ext(header.Filename))
	if extension != ".yaml" && extension != ".yml" {
		return "", "", fmt.Errorf("uploaded file must use a .yaml or .yml extension")
	}
	content, err := io.ReadAll(io.LimitReader(file, migrationUploadLimit+1))
	if err != nil {
		return "", "", fmt.Errorf("read uploaded YAML: %w", err)
	}
	if len(content) > migrationUploadLimit {
		return "", "", fmt.Errorf("uploaded YAML exceeds 2 MiB")
	}
	return string(content), filepath.Base(header.Filename), nil
}

func inspectImportedConfiguration(content string) ([]migrationComponentSummary, []string, error) {
	var document map[string]any
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, nil, fmt.Errorf("parse Collector YAML: %w", err)
	}
	if len(document) == 0 {
		return nil, nil, fmt.Errorf("Collector YAML must be a non-empty mapping")
	}
	sections := []string{"receivers", "processors", "exporters", "extensions", "connectors"}
	sectionLabels := map[string]string{
		"receivers":  "Receivers",
		"processors": "Processors",
		"exporters":  "Exporters",
		"extensions": "Extensions",
		"connectors": "Connectors",
	}
	components := make([]migrationComponentSummary, 0, len(sections)+1)
	for _, section := range sections {
		names, err := importedMappingKeys(document, section)
		if err != nil {
			return nil, nil, err
		}
		components = append(components, migrationComponentSummary{Section: sectionLabels[section], Names: names})
	}
	pipelines := []string{}
	if service, ok := document["service"]; ok {
		serviceMap, ok := service.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("service must be a YAML mapping")
		}
		if rawPipelines, ok := serviceMap["pipelines"]; ok {
			pipelineMap, ok := rawPipelines.(map[string]any)
			if !ok {
				return nil, nil, fmt.Errorf("service.pipelines must be a YAML mapping")
			}
			for name := range pipelineMap {
				pipelines = append(pipelines, name)
			}
			sort.Strings(pipelines)
		}
	}
	components = append(components, migrationComponentSummary{Section: "Service pipelines", Names: pipelines})
	warnings := []string{}
	if len(pipelines) == 0 {
		warnings = append(warnings, "No service pipelines were found. Standardization cannot produce a deployable bundle until at least one pipeline is defined.")
	}
	known := map[string]bool{"receivers": true, "processors": true, "exporters": true, "extensions": true, "connectors": true, "service": true}
	unknown := []string{}
	for key := range document {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		warnings = append(warnings, "Unknown top-level sections will require review during standardization: "+strings.Join(unknown, ", "))
	}
	return components, warnings, nil
}

func importedMappingKeys(document map[string]any, section string) ([]string, error) {
	raw, ok := document[section]
	if !ok || raw == nil {
		return nil, nil
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a YAML mapping", section)
	}
	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
