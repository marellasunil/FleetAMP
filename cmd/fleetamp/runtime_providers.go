package main

import (
	"html/template"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type runtimeProvidersView struct {
	Page      string
	Tab       string
	Providers []runtimes.Descriptor
	Installed []installedComponentView
	Total     int
	Healthy   int
	Error     string
	Query     string
}

type installedComponentView struct {
	InstanceUID string
	Name        string
	Type        string
	Version     string
	Role        string
	Platform    string
	Workload    string
	Health      string
	Management  string
	Location    string
	LastSeen    string
}

const runtimeProvidersHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>OTel Components · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.runtime-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.runtime-card{display:flex;flex-direction:column}.runtime-card .cardbody{display:grid;gap:16px}.runtime-list{display:flex;flex-wrap:wrap;gap:6px}.capability-list{display:grid;gap:10px}.capability{display:grid;grid-template-columns:auto 1fr;gap:10px;align-items:start;padding:10px;border:1px solid var(--line);border-radius:9px;background:#0a1626}.roadmap{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}@media(max-width:1100px){.runtime-grid,.roadmap{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Management / OTel Components</div><div class="pagetitle">OTel Components</div><div class="subtitle">Catalog and inspect the OpenTelemetry components FleetAMP can govern.</div></div><div class="topactions"><span class="badge off">Read-only foundation</span></div></header><div class="content"><nav class="tabs" aria-label="OTel component views"><a class="tab {{if eq .Tab "catalog"}}active{{end}}" href="/otel-components?tab=catalog">Catalog</a><a class="tab {{if eq .Tab "installed"}}active{{end}}" href="/otel-components?tab=installed">Installed</a><span class="tab">Compatibility <span class="soon">Planned</span></span><span class="tab">Installation Guides <span class="soon">Planned</span></span></nav>{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}{{if eq .Tab "catalog"}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Built-in component catalog</div><div class="cardsub">Supported means the workflow exists today. Planned capabilities are contracts only and cannot change a cluster.</div></div></div><div class="cardbody"><div class="runtime-grid">{{range .Providers}}<article class="card runtime-card"><div class="cardhead"><div><div class="cardtitle">{{.Name}}</div><div class="cardsub code">{{.Type}}</div></div><span class="badge ok">built-in</span></div><div class="cardbody"><p>{{.Description}}</p><div><strong>Roles</strong><div class="runtime-list" style="margin-top:7px">{{range .Roles}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Workload modes</strong><div class="runtime-list" style="margin-top:7px">{{range .WorkloadModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Configuration</strong><div class="runtime-list" style="margin-top:7px"><span class="chip">{{.ConfigFormat}}</span></div></div><div><strong>Management modes</strong><div class="runtime-list" style="margin-top:7px">{{range .ManagementModes}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Deployment methods</strong><div class="runtime-list" style="margin-top:7px">{{range .DeploymentMethods}}<span class="chip">{{.}}</span>{{end}}</div></div><div><strong>Capabilities</strong><div class="capability-list" style="margin-top:7px">{{range .Capabilities}}<div class="capability"><span class="badge {{if eq .Support "supported"}}ok{{else}}off{{end}}">{{.Support}}</span><div><strong>{{.Name}}</strong><div class="tiny">{{.Detail}}</div></div></div>{{end}}</div></div></div></article>{{end}}</div></div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">Safety boundary</div><div class="cardsub">Component installation remains intentionally disabled in this milestone.</div></div></div><div class="cardbody roadmap"><div class="component-item"><strong>Discover first</strong><div class="tiny">Inventory identity, version, platform and capabilities without changing workloads.</div></div><div class="component-item"><strong>Validate compatibility</strong><div class="tiny">Check component, Operator, CRD and FleetAMP support before a request is created.</div></div><div class="component-item"><strong>Approve before apply</strong><div class="tiny">Future installs and upgrades enter the same immutable approval workflow as configuration.</div></div></div></section>{{else}}<div class="metrics"><div class="metric"><div class="metriclabel">Installed components</div><div class="metricvalue">{{.Total}}</div><div class="metricnote">Live FleetAMP inventory</div></div><div class="metric"><div class="metriclabel">Healthy</div><div class="metricvalue green">{{.Healthy}}</div><div class="metricnote">Connected and reporting healthy</div></div><div class="metric upcoming"><div class="metriclabel">Compatibility findings</div><div class="metricvalue purple">—</div><div class="metricnote">Planned validation stage</div></div><div class="metric upcoming"><div class="metriclabel">Pending lifecycle requests</div><div class="metricvalue purple">—</div><div class="metricnote">Planned deployment integration</div></div></div><section class="card"><div class="cardhead"><div><div class="cardtitle">Installed component inventory</div><div class="cardsub">Read-only records derived from components registered with FleetAMP through OpAMP.</div></div><form class="toolbar" method="get"><input type="hidden" name="tab" value="installed"><input class="input" type="search" name="q" value="{{.Query}}" placeholder="Search name, host, version or location"><button class="btn" type="submit">Search</button></form></div>{{if .Installed}}<div style="overflow:auto"><table><thead><tr><th>Component</th><th>Type / role</th><th>Version</th><th>Platform</th><th>Workload</th><th>Location</th><th>Management</th><th>Health</th><th>Last seen</th></tr></thead><tbody>{{range .Installed}}<tr><td><a class="agentname" href="/agents/{{.InstanceUID}}">{{.Name}}</a><div class="tiny code">{{.InstanceUID}}</div></td><td>{{.Type}}<div class="tiny">{{.Role}}</div></td><td>{{if .Version}}{{.Version}}{{else}}—{{end}}</td><td>{{.Platform}}</td><td>{{.Workload}}</td><td>{{.Location}}</td><td><span class="chip">{{.Management}}</span></td><td><span class="badge {{if eq .Health "Healthy"}}ok{{else if eq .Health "Warning"}}warn{{else}}off{{end}}">{{.Health}}</span></td><td>{{.LastSeen}}</td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No installed components match this view. Connect an OpenTelemetry Collector through OpAMP to populate the inventory.</div>{{end}}</section><div class="notice" style="margin-top:16px">Installed inventory is observational only. It does not query Kubernetes directly and cannot install, upgrade, restart or remove workloads.</div>{{end}}</div></main></div></body></html>`

var runtimeProvidersPage = template.Must(template.New("runtime-providers").Parse(runtimeProvidersHTML))

func registerRuntimeProviderRoutes(mux *http.ServeMux, registry *runtimes.Registry, agentStore ...*memory.AgentStore) {
	mux.HandleFunc("/otel-components", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/otel-components" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tab := r.URL.Query().Get("tab")
		if tab != "installed" {
			tab = "catalog"
		}
		view := runtimeProvidersView{Page: "otel-components", Tab: tab, Providers: registry.List(), Query: strings.TrimSpace(r.URL.Query().Get("q"))}
		if tab == "installed" && len(agentStore) > 0 && agentStore[0] != nil {
			items, err := agentStore[0].List(r.Context())
			if err != nil {
				view.Error = "Unable to read the installed component inventory."
			} else {
				view.Installed, view.Total, view.Healthy = installedComponentInventory(items, view.Query)
			}
		}
		if err := runtimeProvidersPage.Execute(w, view); err != nil {
			http.Error(w, "render OTel components", http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/runtime-providers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/runtime-providers" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/otel-components", http.StatusPermanentRedirect)
	})
}

func installedComponentInventory(items []*agents.ManagedAgent, query string) ([]installedComponentView, int, int) {
	query = strings.ToLower(strings.TrimSpace(query))
	installed := make([]installedComponentView, 0, len(items))
	healthy := 0
	for _, agent := range items {
		if agent == nil {
			continue
		}
		view := installedComponentFromAgent(agent)
		if view.Health == "Healthy" {
			healthy++
		}
		if query != "" && !strings.Contains(strings.ToLower(strings.Join([]string{view.Name, view.InstanceUID, view.Type, view.Version, view.Platform, view.Location}, " ")), query) {
			continue
		}
		installed = append(installed, view)
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })
	return installed, len(items), healthy
}

func installedComponentFromAgent(agent *agents.ManagedAgent) installedComponentView {
	name := strings.TrimSpace(agent.Name)
	if name == "" {
		name = agent.InstanceUID
	}
	componentType := "Managed telemetry component"
	if agent.Type == agents.AgentTypeOTelCollector {
		componentType = "OTel Collector"
	}
	role := firstAttribute(agent.Attributes, "fleetamp.component.role", "otel.collector.role", "service.role")
	if role == "" {
		role = "Unreported"
	}
	platform := string(agent.Deployment.Runtime)
	if platform == "" || platform == string(agents.RuntimeUnknown) {
		platform = firstAttribute(agent.Attributes, "os.type")
	}
	if arch := firstAttribute(agent.Attributes, "host.arch"); arch != "" {
		platform = strings.Trim(strings.Join([]string{platform, arch}, " / "), " / ")
	}
	if platform == "" {
		platform = "Unknown"
	}
	location := strings.Trim(strings.Join([]string{agent.Deployment.Cluster, agent.Deployment.Namespace}, " / "), " / ")
	if location == "" {
		location = firstAttribute(agent.Attributes, "host.name", "cloud.region")
	}
	if location == "" {
		location = "—"
	}
	lastSeen := "—"
	if !agent.LastSeen.IsZero() {
		lastSeen = agent.LastSeen.UTC().Format(time.RFC3339)
	}
	return installedComponentView{
		InstanceUID: agent.InstanceUID,
		Name:        name,
		Type:        componentType,
		Version:     agent.Version,
		Role:        role,
		Platform:    platform,
		Workload:    componentWorkload(agent),
		Health:      componentHealth(agent),
		Management:  "OpAMP",
		Location:    location,
		LastSeen:    lastSeen,
	}
}

func componentWorkload(agent *agents.ManagedAgent) string {
	if value := firstAttribute(agent.Attributes, "k8s.workload.kind", "fleetamp.workload.mode"); value != "" {
		return value
	}
	for _, workload := range []struct{ key, label string }{
		{"k8s.daemonset.name", "DaemonSet"}, {"k8s.statefulset.name", "StatefulSet"}, {"k8s.deployment.name", "Deployment"},
	} {
		if strings.TrimSpace(agent.Attributes[workload.key]) != "" {
			return workload.label
		}
	}
	switch agent.Deployment.Runtime {
	case agents.RuntimeVM, agents.RuntimeBareMetal:
		return "System service"
	case agents.RuntimeContainer:
		return "Container"
	case agents.RuntimeKubernetes:
		return "Kubernetes workload"
	default:
		return "Unknown"
	}
}

func componentHealth(agent *agents.ManagedAgent) string {
	if agent.Status == agents.LifecycleRetired {
		return "Retired"
	}
	if !agent.Connected {
		return "Offline"
	}
	if agent.Healthy {
		return "Healthy"
	}
	return "Warning"
}

func firstAttribute(attributes map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attributes[key]); value != "" {
			return value
		}
	}
	return ""
}
