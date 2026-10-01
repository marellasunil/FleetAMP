package main

import (
	"html/template"
	"net/http"
)

const guideCSS = `
.guide-map{padding:22px;border:1px solid #29384a;border-radius:14px;background:#090d12;overflow:auto}.guide-flow{position:relative;display:grid;grid-template-columns:minmax(205px,1fr) 42px minmax(180px,1fr) 42px minmax(195px,1fr) 42px minmax(195px,1fr) 42px minmax(195px,1fr);align-items:center;min-width:1080px;min-height:340px}.guide-column{position:relative;z-index:2;display:grid;gap:9px;align-content:center}.guide-heading{margin-bottom:5px;color:var(--muted);font-size:11px;font-weight:800;letter-spacing:.09em;text-transform:uppercase}.guide-node{display:flex;align-items:center;gap:9px;min-height:38px;padding:8px 10px;border:1px solid transparent;border-radius:8px;color:#dce5f1;cursor:pointer}.guide-node:hover{background:#121a24}.guide-node:has(input:checked){border-color:#66a7ff;background:#10213a;color:#fff}.guide-node input{appearance:none;width:14px;height:14px;flex:0 0 auto;margin:0;border:2px solid #dce5f1;border-radius:50%}.guide-node input:checked{border:4px solid #66a7ff;background:#fff}.guide-link{min-height:2px}.guide-column[hidden],.guide-link[hidden],.guide-node[hidden]{display:none}.guide-detail{margin-top:18px;padding:20px;border-left:3px solid #66a7ff;background:#0e151f}.guide-detail h2{margin:0 0 8px;font-size:24px}.guide-detail-grid{display:grid;grid-template-columns:repeat(3,minmax(180px,1fr));gap:12px;margin-top:15px}.guide-detail-item{padding:13px;border:1px solid #29384a;border-radius:9px;background:#111923}.guide-detail-item strong{display:block;margin-bottom:6px}.guide-path{display:flex;gap:7px;align-items:center;flex-wrap:wrap;margin-top:10px}.guide-pill{padding:6px 9px;border:1px solid #31507a;border-radius:999px;background:#132641}.guide-arrow{color:var(--blue)}.guide-connectors{position:absolute;inset:0;width:100%;height:100%;z-index:1;pointer-events:none;overflow:visible}.guide-connector{fill:none;stroke:#526276;stroke-width:1.7;opacity:.78;stroke-dasharray:9 7;animation:guide-draw .55s ease-out both}.guide-connector.selected{stroke:#66a7ff;stroke-width:2.7;opacity:1}.guide-node.branch-target{box-shadow:0 0 0 1px #52627666}.guide-node.branch-target:has(input:checked){box-shadow:0 0 0 1px #66a7ff}@keyframes guide-draw{from{stroke-dashoffset:80;opacity:0}to{stroke-dashoffset:0}}@media(prefers-reduced-motion:reduce){.guide-connector{animation:none}}.guide-result{margin-top:16px}.guide-result li{margin:8px 0}@media(max-width:900px){.guide-map{padding:12px}.guide-detail-grid{grid-template-columns:1fr}}
`

const guideHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Guides</title><style>` + controlPlaneCSS + guideCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Guides</div><div class="pagetitle">Instrumentation Guides</div><div class="subtitle">Choose what you want to observe. FleetAMP reveals one relevant decision at a time.</div></div><div class="topactions"><button class="btn" type="button" data-guide-reset>Start over</button></div></header><div class="content">
<section class="card"><div class="cardhead"><div><div class="cardtitle">Design an OpenTelemetry approach</div><div class="cardsub">Select a radio node to reveal the next connected branch.</div></div></div><div class="cardbody"><div class="guide-map" data-guide><div class="guide-flow"><svg class="guide-connectors" aria-hidden="true"><defs><marker id="guide-arrow-muted" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#526276"></path></marker><marker id="guide-arrow-selected" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="#66a7ff"></path></marker></defs><g data-connectors></g></svg>
<div class="guide-column"><div class="guide-heading">Capability</div>
<label class="guide-node"><input type="radio" name="goal" value="apm"><span>Application performance</span></label>
<label class="guide-node"><input type="radio" name="goal" value="infrastructure"><span>Infrastructure</span></label>
<label class="guide-node"><input type="radio" name="goal" value="kubernetes"><span>Kubernetes</span></label>
<label class="guide-node"><input type="radio" name="goal" value="logs"><span>Logs</span></label>
<label class="guide-node"><input type="radio" name="goal" value="database"><span>Database</span></label>
<label class="guide-node"><input type="radio" name="goal" value="frontend"><span>Frontend</span></label>
<label class="guide-node"><input type="radio" name="goal" value="custom"><span>Custom telemetry</span></label></div>
<div class="guide-link" data-link="platform" hidden></div>
<div class="guide-column" data-column="platform" hidden><div class="guide-heading">Platform</div>
<label class="guide-node" data-goals="apm infrastructure logs database custom"><input type="radio" name="platform" value="linux"><span>Linux / VM</span></label>
<label class="guide-node" data-goals="apm infrastructure kubernetes logs database custom"><input type="radio" name="platform" value="kubernetes"><span>Kubernetes</span></label>
<label class="guide-node" data-goals="apm logs custom"><input type="radio" name="platform" value="serverless"><span>Serverless</span></label>
<label class="guide-node" data-goals="frontend"><input type="radio" name="platform" value="browser"><span>Browser</span></label>
<label class="guide-node" data-goals="apm infrastructure logs database custom"><input type="radio" name="platform" value="other"><span>Other / OTLP</span></label></div>
<div class="guide-link" data-link="technology" hidden></div>
<div class="guide-column" data-column="technology" hidden><div class="guide-heading">Technology or source</div>
<label class="guide-node" data-tech-goals="apm custom"><input type="radio" name="technology" value="java"><span>Java</span></label>
<label class="guide-node" data-tech-goals="apm custom"><input type="radio" name="technology" value="dotnet"><span>.NET</span></label>
<label class="guide-node" data-tech-goals="apm custom"><input type="radio" name="technology" value="python"><span>Python</span></label>
<label class="guide-node" data-tech-goals="apm frontend custom"><input type="radio" name="technology" value="javascript"><span>JavaScript / Node.js</span></label>
<label class="guide-node" data-tech-goals="apm custom"><input type="radio" name="technology" value="go"><span>Go</span></label>
<label class="guide-node" data-tech-goals="infrastructure"><input type="radio" name="technology" value="host"><span>Host metrics</span></label>
<label class="guide-node" data-tech-goals="kubernetes infrastructure"><input type="radio" name="technology" value="cluster"><span>Cluster and workloads</span></label>
<label class="guide-node" data-tech-goals="logs"><input type="radio" name="technology" value="filelog"><span>File / syslog / journald</span></label>
<label class="guide-node" data-tech-goals="database"><input type="radio" name="technology" value="database"><span>SQL / database receiver</span></label>
<label class="guide-node" data-tech-goals="custom"><input type="radio" name="technology" value="generic"><span>Generic OTLP source</span></label></div>
<div class="guide-link" data-link="method" hidden></div>
<div class="guide-column" data-column="method" hidden><div class="guide-heading">Instrumentation or collection</div>
<label class="guide-node" data-method-goals="apm frontend"><input type="radio" name="method" value="auto"><span>Auto-instrumentation</span></label>
<label class="guide-node" data-method-goals="apm frontend custom"><input type="radio" name="method" value="sdk"><span>OpenTelemetry SDK</span></label>
<label class="guide-node" data-method-goals="infrastructure kubernetes logs database custom"><input type="radio" name="method" value="receiver"><span>Collector receiver</span></label>
<label class="guide-node" data-method-goals="apm kubernetes infrastructure"><input type="radio" name="method" value="ebpf"><span>eBPF / OBI</span></label></div>
<div class="guide-link" data-link="topology" hidden></div>
<div class="guide-column" data-column="topology" hidden><div class="guide-heading">Deployment topology</div>
<label class="guide-node" data-platforms="linux other"><input type="radio" name="topology" value="agent"><span>Host agent</span></label>
<label class="guide-node" data-platforms="kubernetes"><input type="radio" name="topology" value="operator"><span>OTel Operator</span></label>
<label class="guide-node" data-platforms="kubernetes"><input type="radio" name="topology" value="daemonset"><span>DaemonSet</span></label>
<label class="guide-node" data-platforms="kubernetes"><input type="radio" name="topology" value="sidecar"><span>Sidecar</span></label>
<label class="guide-node" data-platforms="linux kubernetes serverless browser other"><input type="radio" name="topology" value="gateway"><span>Central gateway</span></label>
<label class="guide-node" data-platforms="serverless browser other"><input type="radio" name="topology" value="direct"><span>Direct OTLP export</span></label></div>
</div>
<div class="guide-detail"><h2 data-title>Select a capability</h2><p data-summary>Only the first decision is shown. Your selection reveals the next connected branch.</p><div class="guide-path" data-path></div><div class="guide-detail-grid"><div class="guide-detail-item"><strong>Advantages</strong><span data-advantages>—</span></div><div class="guide-detail-item"><strong>Requirements</strong><span data-requirements>—</span></div><div class="guide-detail-item"><strong>Considerations</strong><span data-considerations>—</span></div></div></div>
</div></div></section>
<section class="card guide-result" data-result hidden><div class="cardhead"><div><div class="cardtitle">Recommended implementation direction</div><div class="cardsub">Guidance only—this page does not create or deploy configuration.</div></div></div><div class="cardbody"><ol data-steps></ol><div class="notice" style="margin-top:14px">Patterns and Blueprints will be designed separately in a future iteration.</div></div></section>
</div></main></div><script src="/assets/guides.js" defer></script></body></html>`

const guideJS = `
(function(){
 var root=document.querySelector("[data-guide]");if(!root)return;
 function get(n){var x=root.querySelector("[name='"+n+"']:checked");return x?x.value:""}
 function text(n){var x=root.querySelector("[name='"+n+"']:checked");return x?x.closest("label").innerText.trim():""}
 function show(n,on){root.querySelector("[data-column='"+n+"']").hidden=!on;root.querySelector("[data-link='"+n+"']").hidden=!on}
 function filter(selector,key,value){root.querySelectorAll(selector).forEach(function(node){var ok=(node.dataset[key]||"").split(/\s+/).includes(value);node.hidden=!ok;if(!ok)node.querySelector("input").checked=false})}
 var guidance={auto:["Automatic instrumentation","Fast onboarding and framework-aware traces.","Supported runtime, injection method and application restart.","Check runtime compatibility and measure overhead."],sdk:["OpenTelemetry SDK","Maximum control and rich business context.","Application code, build and release changes.","Teams own SDK lifecycle and semantic quality."],receiver:["Collector receiver","Reusable collection with centralized processing.","Source connectivity, credentials and receiver permissions.","Plan interval, volume, cardinality and availability."],ebpf:["eBPF / OBI","Low-friction discovery and broad baseline visibility.","Compatible Linux kernel and elevated capabilities.","Coverage varies and business context is more limited."]};
 var steps={agent:["Install a managed Collector or language agent on the host.","Send OTLP to an approved gateway or backend.","Validate telemetry and Collector self-observability."],operator:["Install the OpenTelemetry Operator with required cluster permissions.","Define Instrumentation and Collector resources.","Roll out gradually and validate injected workloads."],daemonset:["Deploy a Collector DaemonSet on selected nodes.","Configure node-local receivers, enrichment and secure export.","Validate RBAC, resource usage and scheduling coverage."],sidecar:["Add a Collector sidecar to the workload template.","Route application telemetry to the local sidecar.","Account for per-pod resources and joint rollouts."],gateway:["Deploy redundant Collector gateways behind a stable endpoint.","Configure TLS, batching, memory protection and routing.","Load-test capacity and monitor queues and export failures."],direct:["Configure the SDK or runtime for direct OTLP export.","Use TLS and managed endpoint credentials.","Add a gateway when shared processing or buffering is required."]};
 var svgNS="http://www.w3.org/2000/svg",flow=root.querySelector(".guide-flow"),connectorGroup=root.querySelector("[data-connectors]");
 var stages=[["goal","platform"],["platform","technology"],["technology","method"],["method","topology"]];
 function drawConnectors(){connectorGroup.replaceChildren();root.querySelectorAll(".guide-node.branch-target").forEach(function(n){n.classList.remove("branch-target")});var flowRect=flow.getBoundingClientRect();stages.forEach(function(stage){var input=root.querySelector("[name='"+stage[0]+"']:checked"),column=root.querySelector("[data-column='"+stage[1]+"']");if(!input||!column||column.hidden)return;var source=input.closest(".guide-node"),sourceRect=source.getBoundingClientRect();column.querySelectorAll(".guide-node:not([hidden])").forEach(function(target){if(target.offsetParent===null)return;target.classList.add("branch-target");var targetRect=target.getBoundingClientRect(),x1=sourceRect.right-flowRect.left,y1=sourceRect.top+sourceRect.height/2-flowRect.top,x2=targetRect.left-flowRect.left,y2=targetRect.top+targetRect.height/2-flowRect.top,bend=Math.max(20,(x2-x1)*.45),selected=!!target.querySelector("input:checked"),path=document.createElementNS(svgNS,"path");path.setAttribute("d","M "+x1+" "+y1+" C "+(x1+bend)+" "+y1+", "+(x2-bend)+" "+y2+", "+x2+" "+y2);path.setAttribute("class","guide-connector"+(selected?" selected":""));path.setAttribute("marker-end",selected?"url(#guide-arrow-selected)":"url(#guide-arrow-muted)");connectorGroup.append(path)})})}
 function scheduleConnectors(){requestAnimationFrame(drawConnectors)}
 function path(){var target=root.querySelector("[data-path]");target.replaceChildren();["goal","platform","technology","method","topology"].map(text).filter(Boolean).forEach(function(v,i){if(i){var a=document.createElement("span");a.className="guide-arrow";a.textContent="→";target.append(a)}var p=document.createElement("span");p.className="guide-pill";p.textContent=v;target.append(p)})}
 function update(){var goal=get("goal"),platform=get("platform"),technology=get("technology"),method=get("method"),topology=get("topology");filter("[data-goals]","goals",goal);show("platform",!!goal);filter("[data-tech-goals]","techGoals",goal);show("technology",!!platform);filter("[data-method-goals]","methodGoals",goal);show("method",!!technology);filter("[data-platforms]","platforms",platform);show("topology",!!method);path();var title=root.querySelector("[data-title]"),summary=root.querySelector("[data-summary]");if(method){var g=guidance[method];title.textContent=topology?text("topology"):g[0];summary.textContent=topology?g[1]+" Recommended topology: "+text("topology")+".":g[1];root.querySelector("[data-advantages]").textContent=g[1];root.querySelector("[data-requirements]").textContent=g[2];root.querySelector("[data-considerations]").textContent=g[3]}else if(technology){title.textContent=text("technology");summary.textContent="Choose an instrumentation or collection method."}else if(platform){title.textContent=text("platform");summary.textContent="Choose the technology or telemetry source."}else if(goal){title.textContent=text("goal");summary.textContent="Choose where this capability runs."}else{title.textContent="Select a capability";summary.textContent="Only the first decision is shown. Your selection reveals the next connected branch.";root.querySelector("[data-advantages]").textContent="—";root.querySelector("[data-requirements]").textContent="—";root.querySelector("[data-considerations]").textContent="—"}var result=document.querySelector("[data-result]");result.hidden=!topology;if(topology){var list=result.querySelector("[data-steps]");list.replaceChildren();steps[topology].forEach(function(v){var li=document.createElement("li");li.textContent=v;list.append(li)})}scheduleConnectors()}
 root.addEventListener("change",update);window.addEventListener("resize",scheduleConnectors);root.closest(".guide-map").addEventListener("scroll",scheduleConnectors,{passive:true});document.querySelector("[data-guide-reset]").addEventListener("click",function(){root.querySelectorAll("input").forEach(function(x){x.checked=false});update()});update();
})();
`

var guidePage = template.Must(template.New("guide").Parse(guideHTML))

func registerGuideRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /assets/guides.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(guideJS))
	})
	mux.HandleFunc("GET /instrumentation", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = guidePage.Execute(w, upcomingView{Page: "instrumentation"})
	})
}
