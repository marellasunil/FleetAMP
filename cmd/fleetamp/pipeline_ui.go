// Configuration pipeline visualization for immutable Collector artifacts.
package main

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type configurationDetailView struct {
	Page          string
	Configuration *configs.Configuration
	Pipeline      *configs.PipelineModel
	Error         string
}

const pipelineCSS = `
.pipeline-list{display:grid;gap:14px}.pipeline{padding:16px}.pipeline-title{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:13px}.pipeline-flow{display:grid;grid-template-columns:minmax(0,1fr) 26px minmax(0,1fr) 26px minmax(0,1fr);align-items:stretch;gap:8px}.pipeline-stage{background:#0a1524;border:1px solid #243954;border-radius:9px;padding:13px;min-width:0}.pipeline-stage-title{color:var(--muted);font-size:9px;letter-spacing:.12em;text-transform:uppercase;margin-bottom:9px}.pipeline-components{display:flex;flex-wrap:wrap;gap:6px}.pipeline-arrow{display:grid;place-items:center;color:var(--blue);font-size:19px}.pipeline-empty{color:var(--muted);font-size:10px}.warning-list{margin:0;padding-left:18px;color:#ffd08a}.warning-list li+li{margin-top:7px}@media(max-width:850px){.pipeline-flow{grid-template-columns:1fr}.pipeline-arrow{transform:rotate(90deg);min-height:24px}}
`

const configurationDetailHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Configuration Pipeline</title><style>` + controlPlaneCSS + detailCSS + pipelineCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Configuration / Pipeline</div><div class="pagetitle">{{.Configuration.Name}} v{{.Configuration.Version}}</div><div class="subtitle">Collector pipeline structure derived from this immutable configuration</div></div><div class="topactions"><a class="btn" href="/agents">← Fleet</a></div></header>
<div class="content">
<div class="sectionhead"><div><div class="eyebrow">Configuration topology</div><div class="sectiontitle">Pipeline visualization</div><div class="subtitle">Configured collection and delivery path. Component settings and secrets are intentionally hidden.</div></div><div><span class="badge ok">Validated model</span></div></div>
{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{else}}
<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Configuration identity</div><div class="cardsub">Immutable saved artifact</div></div></div><div class="cardbody"><div class="kv"><span>Name</span><span>{{.Configuration.Name}}</span><span>Version</span><span>{{.Configuration.Version}}</span><span>Hash</span><span class="code">{{.Configuration.Hash}}</span><span>Created</span><span>{{.Configuration.CreatedAt}}</span></div></div></section>
{{if .Pipeline.Warnings}}<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle amber">Configuration warnings</div><div class="cardsub">Defined components that are not connected to an active service pipeline</div></div><span class="badge warn">{{len .Pipeline.Warnings}} warning(s)</span></div><div class="cardbody"><ul class="warning-list">{{range .Pipeline.Warnings}}<li>{{.}}</li>{{end}}</ul></div></section>{{end}}
{{if .Pipeline.Extensions}}<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Service extensions</div><div class="cardsub">Extensions enabled by the Collector service</div></div></div><div class="cardbody"><div class="chips">{{range .Pipeline.Extensions}}<span class="chip">{{.}}</span>{{end}}</div></div></section>{{end}}
{{if .Pipeline.Pipelines}}<div class="pipeline-list">{{range .Pipeline.Pipelines}}<section class="card pipeline"><div class="pipeline-title"><div><strong>{{.Name}}</strong><div class="tiny">{{.Signal}} telemetry</div></div><span class="badge ok">{{.Signal}}</span></div><div class="pipeline-flow" aria-label="{{.Name}} pipeline: receivers, processors, exporters"><div class="pipeline-stage"><div class="pipeline-stage-title">Collect · Receivers</div><div class="pipeline-components">{{range .Receivers}}<span class="chip">{{.}}</span>{{end}}</div></div><div class="pipeline-arrow" aria-hidden="true">→</div><div class="pipeline-stage"><div class="pipeline-stage-title">Process · In order</div>{{if .Processors}}<div class="pipeline-components">{{range .Processors}}<span class="chip">{{.}}</span>{{end}}</div>{{else}}<div class="pipeline-empty">No processors configured</div>{{end}}</div><div class="pipeline-arrow" aria-hidden="true">→</div><div class="pipeline-stage"><div class="pipeline-stage-title">Send · Exporters</div><div class="pipeline-components">{{range .Exporters}}<span class="chip">{{.}}</span>{{end}}</div></div></div><div class="tiny" style="margin-top:11px">Plain flow: {{range $index,$item := .Receivers}}{{if $index}}, {{end}}{{$item}}{{end}} → {{if .Processors}}{{range $index,$item := .Processors}}{{if $index}}, {{end}}{{$item}}{{end}} → {{end}}{{range $index,$item := .Exporters}}{{if $index}}, {{end}}{{$item}}{{end}}</div></section>{{end}}</div>{{else}}<section class="card"><div class="empty">This configuration does not define service pipelines.</div></section>{{end}}
{{end}}
</div></main></div></body></html>`

var configurationDetailPage = template.Must(template.New("configuration-pipeline").Parse(configurationDetailHTML))

// registerConfigurationUIRoutes exposes read-only, secret-safe views of saved configurations.
func registerConfigurationUIRoutes(mux *http.ServeMux, configStore storage.ConfigurationStore) {
	mux.HandleFunc("GET /configurations/{id}", func(w http.ResponseWriter, r *http.Request) {
		configuration, err := configStore.Get(r.Context(), r.PathValue("id"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		view := configurationDetailView{Page: "fleet", Configuration: configuration}
		view.Pipeline, err = configs.ParsePipelineModel(configuration.Content)
		if err != nil {
			view.Error = "Unable to visualize this configuration: " + err.Error()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := configurationDetailPage.Execute(w, view); err != nil {
			slog.Error("failed to render configuration pipeline", "component", "http", "event", "render_failed", "page", "configuration_pipeline", "error", err)
		}
	})
}
