package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"gopkg.in/yaml.v3"
)

type blueprintGroup struct {
	Group     *groups.Group
	Reviewers []reviewerOption
}
type blueprintView struct {
	Page           string
	Groups         []blueprintGroup
	Destinations   []*blueprints.DestinationProfile
	Patterns       []*blueprints.Pattern
	Blocks         []*blueprints.Block
	IsAdmin        bool
	Message, Error string
}

const blueprintJourneyCSS = `
.blueprint-journey{display:grid;grid-template-columns:repeat(5,minmax(120px,1fr));gap:8px;margin-bottom:16px}.journey-step{padding:10px;border:1px solid var(--line);border-radius:9px;background:var(--panel2)}.journey-step strong{display:block;color:var(--blue);font-size:12px}.guidance-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(210px,1fr));gap:12px}.recommendation{border:1px solid #31507a;border-radius:10px;padding:13px;background:#0b1b30}.recommendation strong{display:block;margin-bottom:5px}.telemetry-flow{display:grid;grid-template-columns:repeat(5,minmax(120px,1fr));align-items:center;gap:20px;margin:14px 0}.flow-node{position:relative;min-height:82px;padding:12px;border:1px solid #31507a;border-radius:10px;background:#0b1b30;display:grid;align-content:center;text-align:center}.flow-node:not(:last-child)::after{content:"→";position:absolute;right:-17px;top:29px;color:var(--blue);font-size:20px}.implementation-steps{margin:8px 0 0;padding-left:20px}.implementation-steps li{margin:6px 0}@media(max-width:900px){.blueprint-journey,.telemetry-flow{grid-template-columns:1fr}.flow-node:not(:last-child)::after{content:"↓";right:auto;left:50%;top:auto;bottom:-22px}}
`

const blueprintsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Blueprint</title><style>` + controlPlaneCSS + blueprintJourneyCSS + `
.choice-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(165px,1fr));gap:10px}.choice-card{display:flex;gap:10px;align-items:flex-start;padding:13px;border:1px solid var(--line);border-radius:10px;background:var(--panel2);cursor:pointer}.choice-card:has(input:checked){border-color:var(--blue);background:#102344;box-shadow:0 0 0 1px var(--blue)}.choice-card input{margin-top:3px}.decision-canvas{display:grid;gap:18px}.decision-stage{position:relative;padding:16px;border:1px solid #29405e;border-radius:12px;background:var(--panel)}.decision-stage:not(:last-child)::after{content:"↓";position:absolute;left:50%;bottom:-24px;color:var(--blue);font-size:22px;z-index:2}.stage-number{display:inline-grid;place-items:center;width:25px;height:25px;margin-right:8px;border-radius:50%;background:var(--blue);color:white;font-weight:800}.stage-title{font-weight:800}.stage-help{margin:4px 0 12px;color:var(--muted)}.flow-summary{position:sticky;bottom:12px;z-index:20;padding:12px;border:1px solid #31507a;border-radius:12px;background:#09172aec;backdrop-filter:blur(8px)}.flow-path{display:flex;align-items:center;gap:8px;overflow:auto}.flow-pill{padding:7px 10px;border-radius:999px;background:#142a49;white-space:nowrap}.flow-arrow{color:var(--blue)}.guidance-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));gap:10px}.catalog-admin{margin-top:18px}.catalog-admin summary{cursor:pointer;font-weight:800}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Blueprint</div><div class="pagetitle">Design observability visually</div><div class="subtitle">Start with what you want to observe. FleetAMP reveals only the next relevant decision.</div></div><div class="topactions"><div class="connection"><span class="dot"></span>Governed workflow</div></div></header><div class="content">
{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}{{if .Error}}<div class="configerror">{{.Error}}</div>{{end}}
{{if and .Groups .Destinations .Patterns}}<form method="post" action="/blueprints" data-decision-builder><input type="hidden" name="action" value="generate"><div class="decision-canvas">
<section class="decision-stage" data-stage="goal"><div><span class="stage-number">1</span><span class="stage-title">What do you want to observe?</span></div><div class="stage-help">Choose one capability to begin the Blueprint.</div><div class="choice-grid"><label class="choice-card"><input type="radio" name="goal" value="apm" required><span><strong>APM</strong><div class="tiny">Application performance, dependencies and errors</div></span></label><label class="choice-card"><input type="radio" name="goal" value="infrastructure"><span><strong>Infrastructure</strong><div class="tiny">Hosts, CPU, memory, disk and network</div></span></label><label class="choice-card"><input type="radio" name="goal" value="kubernetes"><span><strong>Kubernetes</strong><div class="tiny">Clusters, nodes, workloads and services</div></span></label><label class="choice-card"><input type="radio" name="goal" value="logs"><span><strong>Logs</strong><div class="tiny">Collect, enrich and route logs</div></span></label><label class="choice-card"><input type="radio" name="goal" value="custom"><span><strong>Custom</strong><div class="tiny">Compose an approved telemetry pipeline</div></span></label></div></section>
<section class="decision-stage" data-stage="platform" hidden><div><span class="stage-number">2</span><span class="stage-title">Where does it run?</span></div><div class="stage-help">Platform determines the safe deployment choices.</div><div class="choice-grid"><label class="choice-card"><input type="radio" name="platform" value="linux" required><span><strong>Linux</strong><div class="tiny">Host, VM or bare metal</div></span></label><label class="choice-card"><input type="radio" name="platform" value="kubernetes"><span><strong>Kubernetes</strong><div class="tiny">Pods, nodes and clusters</div></span></label><label class="choice-card"><input type="radio" name="platform" value="application"><span><strong>Application</strong><div class="tiny">Runtime-managed service</div></span></label><label class="choice-card"><input type="radio" name="platform" value="any"><span><strong>Other</strong><div class="tiny">Generic OTLP-compatible workload</div></span></label></div></section>
<section class="decision-stage" data-stage="technology" hidden><div><span class="stage-number">3</span><span class="stage-title">What technology is used?</span></div><div class="choice-grid"><label class="choice-card"><input type="radio" name="technology" value="java" required><span><strong>Java</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="dotnet"><span><strong>.NET</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="python"><span><strong>Python</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="nodejs"><span><strong>Node.js</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="linux"><span><strong>Linux host</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="kubernetes"><span><strong>Kubernetes platform</strong></span></label><label class="choice-card"><input type="radio" name="technology" value="generic"><span><strong>Generic OTLP</strong></span></label></div></section>
<section class="decision-stage" data-stage="approach" hidden><div><span class="stage-number">4</span><span class="stage-title">How should FleetAMP instrument or deploy it?</span></div><div class="stage-help">Only compatible approaches are shown.</div><div class="choice-grid"><label class="choice-card" data-approach data-platforms="application linux" data-goals="apm"><input type="radio" name="instrumentation_method" value="auto-linux" required><span><strong>Auto-instrumentation</strong><div class="tiny">Language agent, no source changes</div></span></label><label class="choice-card" data-approach data-platforms="application linux kubernetes any" data-goals="apm custom"><input type="radio" name="instrumentation_method" value="sdk"><span><strong>OpenTelemetry SDK</strong><div class="tiny">Explicit code-level telemetry</div></span></label><label class="choice-card" data-approach data-platforms="linux" data-goals="infrastructure logs custom"><input type="radio" name="instrumentation_method" value="collector-agent"><span><strong>Collector agent</strong><div class="tiny">One managed Collector per Linux host</div></span></label><label class="choice-card" data-approach data-platforms="kubernetes" data-goals="apm kubernetes"><input type="radio" name="instrumentation_method" value="operator"><span><strong>OTel Operator</strong><div class="tiny">Declarative injection with CRDs</div></span></label><label class="choice-card" data-approach data-platforms="kubernetes" data-goals="kubernetes infrastructure logs"><input type="radio" name="instrumentation_method" value="daemonset"><span><strong>DaemonSet</strong><div class="tiny">Node-local collection</div></span></label><label class="choice-card" data-approach data-platforms="kubernetes" data-goals="apm logs custom"><input type="radio" name="instrumentation_method" value="sidecar"><span><strong>Sidecar</strong><div class="tiny">Collector beside every workload</div></span></label><label class="choice-card" data-approach data-platforms="kubernetes application any" data-goals="apm kubernetes infrastructure logs custom"><input type="radio" name="instrumentation_method" value="gateway"><span><strong>Gateway</strong><div class="tiny">Central scalable processing</div></span></label><label class="choice-card" data-approach data-platforms="linux kubernetes" data-goals="apm kubernetes"><input type="radio" name="instrumentation_method" value="ebpf"><span><strong>eBPF / OBI</strong><div class="tiny">Kernel-based discovery and telemetry</div></span></label></div><div class="recommendation" data-approach-details hidden style="margin-top:12px"><strong data-detail-title></strong><div class="guidance-grid"><div><span class="tiny">What it is</span><p data-detail-summary></p></div><div><span class="tiny">Advantages</span><ul data-detail-advantages></ul></div><div><span class="tiny">Requirements</span><ul data-detail-requirements></ul></div><div><span class="tiny">Considerations</span><ul data-detail-limitations></ul></div></div></div></section>
<section class="decision-stage" data-stage="signals" hidden><div><span class="stage-number">5</span><span class="stage-title">Which telemetry signals?</span></div><div class="choice-grid"><label class="choice-card"><input type="checkbox" name="signal" value="metrics" checked><span><strong>Metrics</strong></span></label><label class="choice-card"><input type="checkbox" name="signal" value="traces" checked><span><strong>Traces</strong></span></label><label class="choice-card"><input type="checkbox" name="signal" value="logs" checked><span><strong>Logs</strong></span></label></div></section>
<section class="decision-stage" data-stage="design" hidden><div><span class="stage-number">6</span><span class="stage-title">Recommended governed design</span></div><div class="detailform"><label>Pattern<select class="select" name="pattern" required data-pattern>{{range .Patterns}}{{if .Enabled}}<option value="{{.ID}}" data-platform="{{.Platform}}">{{.Name}} · {{.Platform}}</option>{{end}}{{end}}</select></label><label>Destination<select class="select" name="destination_id" required>{{range .Destinations}}{{if .Enabled}}<option value="{{.ID}}">{{.Name}} · {{.Environment}}</option>{{end}}{{end}}</select></label><label>Target group<select class="select" name="group_id" required>{{range .Groups}}<option value="{{.Group.ID}}">{{.Group.Name}}</option>{{end}}</select></label></div><div class="choice-grid" style="margin-top:12px">{{range .Blocks}}{{if .Enabled}}<label class="choice-card" data-block data-platforms="{{range .Platforms}}{{.}} {{end}}" data-signals="{{range .Signals}}{{.}} {{end}}"><input type="checkbox" name="block_id" value="{{.ID}}" {{if .Required}}checked disabled{{end}}><span><strong>{{.Name}}</strong><div class="tiny">{{.Kind}} · {{.ComponentID}}{{if .Locked}} · Locked{{end}}</div></span>{{if .Required}}<input type="hidden" name="block_id" value="{{.ID}}">{{end}}</label>{{end}}{{end}}</div></section>
<section class="decision-stage" data-stage="govern" hidden><div><span class="stage-number">7</span><span class="stage-title">Version, validate and request approval</span></div><div class="detailform"><label>Configuration name<input class="input" name="name" required maxlength="120"></label><label>Version<input class="input" name="version" required maxlength="60"></label><label>Reviewer<select class="select" name="assigned_reviewer"><option value="">Select for approval</option>{{range .Groups}}{{range .Reviewers}}<option value="{{.Username}}">{{.Username}} · {{.Role}}</option>{{end}}{{end}}</select></label><label>Approval validity<select class="select" name="expiry_days"><option value="7">7 days</option><option value="30" selected>30 days</option><option value="60">60 days</option><option value="90">90 days</option></select></label><label style="grid-column:1/-1">Change reason<textarea class="input" name="change_reason" rows="3"></textarea></label></div><div class="detailactions"><button class="btn" type="submit">Validate and save version</button><button class="btn primary" type="submit" name="submit_for_approval" value="true">Validate, save & request approval</button></div></section>
</div><div class="flow-summary"><div class="tiny">Current Blueprint path</div><div class="flow-path" data-flow-path><span class="tiny">Select a capability to begin</span></div></div></form>{{else}}<div class="card empty">Create an enabled group, Pattern and destination before designing a Blueprint.</div>{{end}}
{{if .IsAdmin}}<details class="card catalog-admin"><summary class="cardhead">Admin Pattern & Block catalog</summary><div class="cardbody"><div class="detailgrid"><form class="detailform" method="post" action="/blueprints"><input type="hidden" name="action" value="create_pattern"><label>Pattern name<input class="input" name="pattern_name" required></label><label>Platform<select class="select" name="platform"><option>linux</option><option>kubernetes</option><option>application</option><option>any</option></select></label><label>Receiver ID<input class="input code" name="receiver_id" required></label><label>Signals<select class="select" name="pattern_signals" multiple><option value="metrics">Metrics</option><option value="traces">Traces</option><option value="logs">Logs</option></select></label><label>Description<input class="input" name="pattern_description"></label><label>Receiver YAML<textarea class="input code" name="receiver_config" required rows="4"></textarea></label><button class="btn" type="submit">Add Pattern</button></form><form class="detailform" method="post" action="/blueprints"><input type="hidden" name="action" value="create_block"><label>Block name<input class="input" name="block_name" required></label><label>Kind<select class="select" name="block_kind"><option value="processors">Processor</option><option value="receivers">Receiver</option><option value="extensions">Extension</option><option value="connectors">Connector</option></select></label><label>Component ID<input class="input code" name="component_id" required></label><label>Description<input class="input" name="block_description"></label><label>Signals<select class="select" name="block_signals" multiple><option value="metrics">Metrics</option><option value="traces">Traces</option><option value="logs">Logs</option></select></label><label>Platforms<select class="select" name="block_platforms" multiple><option value="linux">Linux</option><option value="kubernetes">Kubernetes</option><option value="application">Application</option><option value="any">Any</option></select></label><label>Block YAML<textarea class="input code" name="block_config" required rows="4"></textarea></label><label><input type="checkbox" name="required" value="true"> Required</label><label><input type="checkbox" name="locked" value="true"> Locked</label><button class="btn" type="submit">Add Block</button></form></div><div style="overflow:auto"><table><thead><tr><th>Pattern</th><th>Platform</th><th>Receiver</th></tr></thead><tbody>{{range .Patterns}}<tr><td>{{.Name}}</td><td>{{.Platform}}</td><td class="code">{{.ReceiverID}}</td></tr>{{end}}</tbody></table></div><div style="overflow:auto;margin-top:12px"><table><thead><tr><th>Block</th><th>Kind</th><th>Policy</th></tr></thead><tbody>{{range .Blocks}}<tr><td>{{.Name}}</td><td class="code">{{.Kind}}/{{.ComponentID}}</td><td>{{if .Required}}Required{{else}}Optional{{end}}{{if .Locked}} · Locked{{end}}</td></tr>{{end}}</tbody></table></div></div></details>{{end}}
</div></main></div><script>
(() => {
 const form=document.querySelector('[data-decision-builder]'); if(!form)return;
 const stages=[...form.querySelectorAll('[data-stage]')];
 const value=(name)=>{const item=form.querySelector('[name="'+name+'"]:checked');return item?item.value:''};
 const text=(name)=>{const item=form.querySelector('[name="'+name+'"]:checked');return item?item.closest('label').querySelector('strong').textContent:''};
 const approachInfo={
  'auto-linux':['Language agent instruments the process without source changes.',['Fast onboarding','Rich framework spans'],['Supported runtime','Process restart'],['Per-process rollout','Test runtime overhead']],
  sdk:['Application code uses OpenTelemetry APIs and SDKs.',['Maximum control','Business context'],['Code and build changes','Semantic conventions'],['More engineering effort','SDK lifecycle ownership']],
  'collector-agent':['Collector runs as a service on each Linux host.',['Local buffering','Host receivers'],['Host install','Required permissions'],['Per-host resources','Upgrade ownership']],
  operator:['Operator injects language instrumentation into Kubernetes workloads.',['Declarative rollout','Consistent configuration'],['Operator CRDs','Admission webhook'],['Cluster permissions','Runtime support varies']],
  daemonset:['Collector runs once on every Kubernetes node.',['Node-local logs and metrics','Automatic node coverage'],['RBAC and mounts','Node capacity'],['Not for singleton receivers','Possible privileged access']],
  sidecar:['Collector container runs beside each application container.',['Workload isolation','Local endpoint'],['Pod mutation','Resources per pod'],['Higher cost','Rollout with application']],
  gateway:['Central Collector deployment receives and processes telemetry.',['Shared processing','Scalable routing'],['HA service','TLS and capacity planning'],['Network dependency','Shared bottleneck risk']],
  ebpf:['Kernel instrumentation discovers services with little application change.',['Low-friction discovery','Broad baseline'],['Compatible kernel','Elevated capabilities'],['Less business context','Coverage varies']]
 };
 const show=(name,visible)=>{const stage=form.querySelector('[data-stage="'+name+'"]');stage.hidden=!visible};
 const update=()=>{
  const goal=value('goal'),platform=value('platform'),technology=value('technology'),approach=value('instrumentation_method');
  show('platform',!!goal);show('technology',!!platform);show('approach',!!technology);
  form.querySelectorAll('[data-approach]').forEach(card=>{const ok=card.dataset.platforms.split(/\s+/).includes(platform)&&card.dataset.goals.split(/\s+/).includes(goal);card.hidden=!ok;if(!ok)card.querySelector('input').checked=false});
  show('signals',!!approach);show('design',!!approach);show('govern',!!approach);
  const detail=form.querySelector('[data-approach-details]');
  if(approach){const info=approachInfo[approach];detail.hidden=false;detail.querySelector('[data-detail-title]').textContent=text('instrumentation_method');detail.querySelector('[data-detail-summary]').textContent=info[0];[['[data-detail-advantages]',info[1]],['[data-detail-requirements]',info[2]],['[data-detail-limitations]',info[3]]].forEach(([sel,items])=>{const el=detail.querySelector(sel);el.replaceChildren();items.forEach(v=>{const li=document.createElement('li');li.textContent=v;el.append(li)})})}else detail.hidden=true;
  const pattern=form.querySelector('[data-pattern]');[...pattern.options].forEach(o=>o.hidden=o.dataset.platform!==platform&&o.dataset.platform!=='any'&&!(goal==='apm'&&o.dataset.platform==='application'));if(pattern.selectedOptions[0]&&pattern.selectedOptions[0].hidden){const first=[...pattern.options].find(o=>!o.hidden);if(first)first.selected=true}
  const signals=new Set([...form.querySelectorAll('[name="signal"]:checked')].map(i=>i.value));form.querySelectorAll('[data-block]').forEach(card=>{const p=card.dataset.platforms.split(/\s+/),sig=card.dataset.signals.split(/\s+/);card.hidden=!(p.includes('any')||p.includes(platform))||![...signals].some(v=>sig.includes(v))});
  const path=[text('goal'),text('platform'),text('technology'),text('instrumentation_method')].filter(Boolean);const target=form.querySelector('[data-flow-path]');target.replaceChildren();if(!path.length){target.innerHTML='<span class="tiny">Select a capability to begin</span>'}else path.forEach((v,i)=>{if(i){const a=document.createElement('span');a.className='flow-arrow';a.textContent='→';target.append(a)}const pill=document.createElement('span');pill.className='flow-pill';pill.textContent=v;target.append(pill)});
 };
 form.addEventListener('change',update);update();
})();
</script></body></html>`

var blueprintsPage = template.Must(template.New("blueprints").Parse(blueprintsHTML))

func registerBlueprintRoutes(mux *http.ServeMux, destinations storage.DestinationProfileStore, groupStore storage.GroupStore, agentStore *memory.AgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, requests storage.GroupDeploymentRequestStore, validator *configs.Validator, auth *authManager, notifier *approvalNotifier) {
	mux.HandleFunc("/blueprints", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blueprints" {
			http.NotFound(w, r)
			return
		}
		roleNow := currentRole(auth, r)
		username := currentUsername(auth, r)
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			action := r.FormValue("action")
			if action == "create_pattern" || action == "delete_pattern" || action == "create_block" || action == "delete_block" {
				if roleNow != roleAdmin {
					http.Error(w, "administrator access required", http.StatusForbidden)
					return
				}
				if action == "delete_pattern" {
					if err := destinations.DeletePattern(r.Context(), strings.TrimSpace(r.FormValue("pattern_id"))); err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
					http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Pattern deleted"), http.StatusSeeOther); return
				}
				if action == "delete_block" {
					if err := destinations.DeleteBlock(r.Context(), strings.TrimSpace(r.FormValue("block_id"))); err != nil { http.Error(w, err.Error(), http.StatusNotFound); return }
					http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Component block deleted"), http.StatusSeeOther); return
				}
				if action == "create_pattern" {
					name, platform := strings.TrimSpace(r.FormValue("pattern_name")), strings.TrimSpace(r.FormValue("platform"))
					receiverID, receiverConfig := strings.TrimSpace(r.FormValue("receiver_id")), strings.TrimSpace(r.FormValue("receiver_config"))
					if name == "" || platform == "" || receiverID == "" { http.Error(w, "pattern name, platform and receiver are required", http.StatusUnprocessableEntity); return }
					var cfg map[string]any
					if err := yaml.Unmarshal([]byte(receiverConfig), &cfg); err != nil || cfg == nil { http.Error(w, "receiver configuration must be a YAML mapping", http.StatusUnprocessableEntity); return }
					p := blueprints.NewPattern(name, r.FormValue("pattern_description"), platform, receiverID, receiverConfig, r.Form["pattern_signals"])
					if err := destinations.CreatePattern(r.Context(), p); err != nil { http.Error(w, err.Error(), http.StatusConflict); return }
					http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Pattern added"), http.StatusSeeOther); return
				}
				name, kind := strings.TrimSpace(r.FormValue("block_name")), strings.TrimSpace(r.FormValue("block_kind"))
				componentID, configYAML := strings.TrimSpace(r.FormValue("component_id")), strings.TrimSpace(r.FormValue("block_config"))
				if name == "" || componentID == "" || configYAML == "" { http.Error(w, "block name, component ID and configuration are required", http.StatusUnprocessableEntity); return }
				switch kind { case "receivers", "processors", "extensions", "connectors": default: http.Error(w, "unsupported block kind", http.StatusUnprocessableEntity); return }
				var cfg map[string]any
				if err := yaml.Unmarshal([]byte(configYAML), &cfg); err != nil || cfg == nil { http.Error(w, "block configuration must be a YAML mapping", http.StatusUnprocessableEntity); return }
				block := blueprints.NewBlock(name, r.FormValue("block_description"), kind, componentID, configYAML, r.Form["block_signals"], r.Form["block_platforms"], r.FormValue("required") == "true", r.FormValue("locked") == "true")
				if err := destinations.CreateBlock(r.Context(), block); err != nil { http.Error(w, err.Error(), http.StatusConflict); return }
				http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Component block added"), http.StatusSeeOther); return
			}
			if action == "create_destination" || action == "delete_destination" {
				if roleNow != roleAdmin {
					http.Error(w, "administrator access required", http.StatusForbidden)
					return
				}
				if action == "delete_destination" {
					if err := destinations.Delete(r.Context(), strings.TrimSpace(r.FormValue("destination_id"))); err != nil {
						http.Error(w, err.Error(), http.StatusNotFound)
						return
					}
					http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Destination profile deleted"), http.StatusSeeOther)
					return
				}
				name := strings.TrimSpace(r.FormValue("destination_name"))
				environment := strings.TrimSpace(r.FormValue("environment"))
				exporterID := strings.TrimSpace(r.FormValue("exporter_id"))
				exporterConfig := strings.TrimSpace(r.FormValue("exporter_config"))
				if name == "" || environment == "" || exporterID == "" || exporterConfig == "" {
					http.Error(w, "all destination fields are required", http.StatusUnprocessableEntity)
					return
				}
				var cfg map[string]any
				if err := yaml.Unmarshal([]byte(exporterConfig), &cfg); err != nil || len(cfg) == 0 {
					http.Error(w, "exporter configuration must be a non-empty YAML mapping", http.StatusUnprocessableEntity)
					return
				}
				encryptedConfig, err := encryptDestinationConfig(auth.pepper, exporterConfig)
				if err != nil {
					internalServerError(w, err)
					return
				}
				p := blueprints.NewDestinationProfile(name, environment, exporterID, encryptedConfig)
				if err := destinations.Create(r.Context(), p); err != nil {
					http.Error(w, err.Error(), http.StatusConflict)
					return
				}
				http.Redirect(w, r, "/blueprints?message="+url.QueryEscape("Destination profile added"), http.StatusSeeOther)
				return
			}
			if action != "generate" {
				http.Error(w, "unsupported action", http.StatusBadRequest)
				return
			}
			group, err := groupStore.Get(r.Context(), strings.TrimSpace(r.FormValue("group_id")))
			if err != nil {
				http.Error(w, "group not found", http.StatusNotFound)
				return
			}
			if !requireGroupAccess(w, r, auth, group) {
				return
			}
			if !group.Enabled {
				http.Error(w, "group must be enabled", http.StatusConflict)
				return
			}
			destination, err := destinations.Get(r.Context(), strings.TrimSpace(r.FormValue("destination_id")))
			if err != nil || !destination.Enabled {
				http.Error(w, "enabled destination profile not found", http.StatusNotFound)
				return
			}
			destination.ExporterConfig, err = decryptDestinationConfig(auth.pepper, destination.ExporterConfig)
			if err != nil {
				internalServerError(w, fmt.Errorf("decrypt destination profile: %w", err))
				return
			}
			goal := strings.TrimSpace(r.FormValue("goal"))
			platform := strings.TrimSpace(r.FormValue("platform"))
			technology := strings.TrimSpace(r.FormValue("technology"))
			method := strings.TrimSpace(r.FormValue("instrumentation_method"))
			if goal == "" || platform == "" || technology == "" || method == "" {
				http.Error(w, "goal, platform, technology and instrumentation method are required", http.StatusUnprocessableEntity)
				return
			}
			if !blueprintApproachAllowed(goal, platform, method) {
				http.Error(w, "selected deployment or instrumentation approach is not compatible with the goal and platform", http.StatusUnprocessableEntity)
				return
			}
			pattern, err := destinations.GetPattern(r.Context(), strings.TrimSpace(r.FormValue("pattern")))
			if err != nil || !pattern.Enabled { http.Error(w, "enabled Pattern not found", http.StatusNotFound); return }
			if pattern.Platform != "any" && pattern.Platform != platform && !(goal == "apm" && pattern.Platform == "application") {
				http.Error(w, "selected Pattern is not compatible with the chosen capability and platform", http.StatusUnprocessableEntity)
				return
			}
			allBlocks, err := destinations.ListBlocks(r.Context())
			if err != nil { internalServerError(w, err); return }
			selected := map[string]bool{}
			for _, id := range r.Form["block_id"] { selected[id] = true }
			var blocks []*blueprints.Block
			for _, block := range allBlocks {
				if block.Enabled && (block.Required || block.Locked || selected[block.ID]) { blocks = append(blocks, block) }
			}
			content, err := generateBlueprintYAML(pattern, r.Form["signal"], blocks, destination)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			name, version := strings.TrimSpace(r.FormValue("name")), strings.TrimSpace(r.FormValue("version"))
			if name == "" || version == "" {
				http.Error(w, "configuration name and version are required", http.StatusUnprocessableEntity)
				return
			}
			configuration := configs.NewGroupConfiguration(group.ID, name, version, content, "text/yaml")
			if err := validateConfigurationForApproval(r.Context(), validator, configuration); err != nil {
				http.Error(w, "generated configuration failed validation: "+err.Error(), http.StatusUnprocessableEntity)
				return
			}
			if err := configStore.Put(r.Context(), configuration); err != nil {
				internalServerError(w, err)
				return
			}
			if r.FormValue("submit_for_approval") != "true" {
				http.Redirect(w, r, "/groups/"+group.ID+"?configuration_saved="+url.QueryEscape(configuration.ID), http.StatusSeeOther)
				return
			}
			changeReason := strings.TrimSpace(r.FormValue("change_reason"))
			reviewer := strings.TrimSpace(r.FormValue("assigned_reviewer"))
			if changeReason == "" {
				http.Error(w, "change reason is required for approval", http.StatusUnprocessableEntity)
				return
			}
			if err := validateAssignedReviewer(r.Context(), auth, group, username, reviewer); err != nil {
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
				return
			}
			members, err := membersForGroupIdentity(r.Context(), group, agentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			preview, eligible, err := previewGroupConfiguration(r.Context(), members, true, configuration, assignmentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			if eligible == 0 {
				http.Error(w, "no ready target is eligible for this configuration", http.StatusConflict)
				return
			}
			targets := make([]configs.GroupDeploymentTarget, 0, len(preview))
			for _, item := range preview {
				targets = append(targets, configs.GroupDeploymentTarget{AgentInstanceUID: item.Agent.InstanceUID, AgentName: item.Agent.Name, Readiness: item.Reason, Eligible: item.Reason == "Ready"})
			}
			req, err := configs.NewGroupDeploymentRequest(group.ID, group.Name, group.Selector, configuration, targets, username, reviewer, changeReason)
			if err != nil {
				internalServerError(w, err)
				return
			}
			days, err := approvalExpiryDays(r.FormValue("expiry_days"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			req.ExpiresAt = req.CreatedAt.Add(time.Duration(days) * 24 * time.Hour)
			req.BaseConfigurationID, req.BaseConfigurationHash, err = commonAppliedConfiguration(r.Context(), preview, assignmentStore)
			if err != nil {
				internalServerError(w, err)
				return
			}
			if err := requests.Create(r.Context(), req); err != nil {
				internalServerError(w, err)
				return
			}
			notifier.notify(r.Context(), "submitted", req, group)
			http.Redirect(w, r, "/approvals/"+req.ID, http.StatusSeeOther)
			return
		}
		allGroups, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		visible := groupsVisibleToUser(r.Context(), auth, username, roleNow, allGroups)
		profiles, err := destinations.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		if err := ensureDefaultBlueprintCatalog(r.Context(), destinations); err != nil { internalServerError(w, err); return }
		patterns, err := destinations.ListPatterns(r.Context())
		if err != nil { internalServerError(w, err); return }
		blocks, err := destinations.ListBlocks(r.Context())
		if err != nil { internalServerError(w, err); return }
		items := make([]blueprintGroup, 0, len(visible))
		for _, g := range visible {
			reviewers, e := eligibleDeploymentReviewers(r.Context(), auth, g, username, roleNow)
			if e != nil {
				internalServerError(w, e)
				return
			}
			items = append(items, blueprintGroup{Group: g, Reviewers: reviewers})
		}
		if err := blueprintsPage.Execute(w, blueprintView{Page: "blueprints", Groups: items, Destinations: profiles, Patterns: patterns, Blocks: blocks, IsAdmin: roleNow == roleAdmin, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error")}); err != nil {
			internalServerError(w, err)
		}
	})
}

const destinationConfigPrefix = "enc:v1:"

func destinationEncryptionKey(pepper []byte) []byte {
	digest := sha256.Sum256(append([]byte("fleetamp-destination-profile-v1:"), pepper...))
	return digest[:]
}

func encryptDestinationConfig(pepper []byte, plaintext string) (string, error) {
	if len(pepper) < 32 {
		return "", fmt.Errorf("server pepper is required to encrypt destination profiles")
	}
	block, err := aes.NewCipher(destinationEncryptionKey(pepper))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate destination encryption nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(destinationConfigPrefix))
	return destinationConfigPrefix + base64.RawStdEncoding.EncodeToString(sealed), nil
}

func decryptDestinationConfig(pepper []byte, stored string) (string, error) {
	if !strings.HasPrefix(stored, destinationConfigPrefix) {
		return "", fmt.Errorf("destination profile is not encrypted")
	}
	encoded := strings.TrimPrefix(stored, destinationConfigPrefix)
	sealed, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode encrypted destination profile: %w", err)
	}
	block, err := aes.NewCipher(destinationEncryptionKey(pepper))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(sealed) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted destination profile is truncated")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(destinationConfigPrefix))
	if err != nil {
		return "", fmt.Errorf("authenticate encrypted destination profile: %w", err)
	}
	return string(plaintext), nil
}

func ensureDefaultBlueprintCatalog(ctx context.Context, store storage.DestinationProfileStore) error {
	patterns, err := store.ListPatterns(ctx)
	if err != nil { return err }
	if len(patterns) == 0 {
		defaults := []*blueprints.Pattern{
			blueprints.NewPattern("OTLP application service", "Receive application metrics, traces and logs over OTLP.", "application", "otlp", "protocols:\n  grpc: {}\n  http: {}", []string{"metrics","traces","logs"}),
			blueprints.NewPattern("Host observability", "Collect host CPU, memory, disk, filesystem and network metrics.", "linux", "hostmetrics", "collection_interval: 30s\nscrapers:\n  cpu: {}\n  memory: {}\n  disk: {}\n  filesystem: {}\n  network: {}", []string{"metrics"}),
		}
		for _, p := range defaults { if err := store.CreatePattern(ctx, p); err != nil { return err } }
	}
	blocks, err := store.ListBlocks(ctx)
	if err != nil { return err }
	if len(blocks) == 0 {
		defaults := []*blueprints.Block{
			blueprints.NewBlock("Memory limiter", "Protect the Collector from memory exhaustion.", "processors", "memory_limiter", "check_interval: 1s\nlimit_mib: 512\nspike_limit_mib: 128", []string{"metrics","traces","logs"}, []string{"any"}, true, true),
			blueprints.NewBlock("Batch processor", "Batch telemetry before export.", "processors", "batch", "timeout: 5s", []string{"metrics","traces","logs"}, []string{"any"}, true, true),
		}
		for _, b := range defaults { if err := store.CreateBlock(ctx, b); err != nil { return err } }
	}
	return nil
}

func blueprintApproachAllowed(goal, platform, method string) bool {
	allowed := map[string]map[string][]string{
		"auto-linux": {"apm": {"application", "linux"}},
		"sdk": {"apm": {"application", "linux", "kubernetes", "any"}, "custom": {"application", "linux", "kubernetes", "any"}},
		"collector-agent": {"infrastructure": {"linux"}, "logs": {"linux"}, "custom": {"linux"}},
		"operator": {"apm": {"kubernetes"}, "kubernetes": {"kubernetes"}},
		"daemonset": {"kubernetes": {"kubernetes"}, "infrastructure": {"kubernetes"}, "logs": {"kubernetes"}},
		"sidecar": {"apm": {"kubernetes"}, "logs": {"kubernetes"}, "custom": {"kubernetes"}},
		"gateway": {"apm": {"application", "kubernetes", "any"}, "kubernetes": {"kubernetes"}, "infrastructure": {"kubernetes", "any"}, "logs": {"kubernetes", "application", "any"}, "custom": {"kubernetes", "application", "any"}},
		"ebpf": {"apm": {"linux"}, "kubernetes": {"kubernetes"}},
	}
	goals, ok := allowed[method]
	if !ok { return false }
	for _, candidate := range goals[goal] { if candidate == platform { return true } }
	return false
}

func containsCatalogValue(values []string, wanted string) bool {
	for _, value := range values { if value == wanted || value == "any" { return true } }
	return false
}

func generateBlueprintYAML(pattern *blueprints.Pattern, selected []string, blocks []*blueprints.Block, destination *blueprints.DestinationProfile) (string, error) {
	if pattern == nil { return "", fmt.Errorf("Pattern is required") }
	if destination == nil { return "", fmt.Errorf("destination profile is required") }
	var exporter map[string]any
	if err := yaml.Unmarshal([]byte(destination.ExporterConfig), &exporter); err != nil { return "", fmt.Errorf("invalid destination profile: %w", err) }
	var receiverConfig map[string]any
	if err := yaml.Unmarshal([]byte(pattern.ReceiverConfig), &receiverConfig); err != nil { return "", fmt.Errorf("invalid Pattern receiver: %w", err) }
	signalSet := map[string]bool{}
	for _, signal := range selected {
		if containsCatalogValue(pattern.Signals, signal) { signalSet[signal] = true }
	}
	if len(signalSet) == 0 && len(pattern.Signals) == 1 { signalSet[pattern.Signals[0]] = true }
	if len(signalSet) == 0 { return "", fmt.Errorf("select at least one signal supported by the Pattern") }
	receivers := map[string]any{pattern.ReceiverID: receiverConfig}
	processors := map[string]any{}
	extensions := map[string]any{}
	connectors := map[string]any{}
	processorIDs := []string{}
	extensionIDs := []string{}
	extraReceivers := map[string][]string{}
	for _, block := range blocks {
		if block == nil || !block.Enabled || !containsCatalogValue(block.Platforms, pattern.Platform) { continue }
		var cfg map[string]any
		if err := yaml.Unmarshal([]byte(block.ConfigYAML), &cfg); err != nil { return "", fmt.Errorf("invalid block %s: %w", block.Name, err) }
		switch block.Kind {
		case "processors":
			processors[block.ComponentID] = cfg
			processorIDs = append(processorIDs, block.ComponentID)
		case "receivers":
			receivers[block.ComponentID] = cfg
			for signal := range signalSet { if containsCatalogValue(block.Signals, signal) { extraReceivers[signal] = append(extraReceivers[signal], block.ComponentID) } }
		case "extensions":
			extensions[block.ComponentID] = cfg
			extensionIDs = append(extensionIDs, block.ComponentID)
		case "connectors":
			connectors[block.ComponentID] = cfg
		default:
			return "", fmt.Errorf("unsupported block kind %q", block.Kind)
		}
	}
	pipelines := map[string]any{}
	for _, signal := range []string{"metrics","traces","logs"} {
		if !signalSet[signal] { continue }
		pipelineReceivers := append([]string{pattern.ReceiverID}, extraReceivers[signal]...)
		pipelines[signal] = map[string]any{"receivers": pipelineReceivers, "processors": processorIDs, "exporters": []string{destination.ExporterID}}
	}
	doc := map[string]any{
		"receivers": receivers, "processors": processors,
		"exporters": map[string]any{destination.ExporterID: exporter},
		"extensions": extensions, "connectors": connectors,
		"service": map[string]any{"extensions": extensionIDs, "pipelines": pipelines, "telemetry": map[string]any{"logs": map[string]any{"level": "info"}}},
	}
	b, err := yaml.Marshal(doc)
	if err != nil { return "", err }
	return string(b), nil
}

