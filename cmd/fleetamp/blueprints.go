package main

import (
	"fmt"
	"html/template"
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

type blueprintPattern struct{ ID, Name, Description string }

var blueprintPatterns = []blueprintPattern{
	{ID: "otlp-service", Name: "OTLP application service", Description: "Receive application metrics, traces and logs over OTLP."},
	{ID: "host-observability", Name: "Host observability", Description: "Collect host CPU, memory, disk, filesystem and network metrics."},
}

type blueprintGroup struct {
	Group     *groups.Group
	Reviewers []reviewerOption
}
type blueprintView struct {
	Page           string
	Groups         []blueprintGroup
	Destinations   []*blueprints.DestinationProfile
	Patterns       []blueprintPattern
	IsAdmin        bool
	Message, Error string
}

const blueprintsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Blueprints</title><style>` + controlPlaneCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Blueprints</div><div class="pagetitle">Blueprints</div><div class="subtitle">Build governed OpenTelemetry configurations without writing YAML, then validate and submit them through approval.</div></div><div class="topactions"><div class="connection"><span class="dot"></span>v0.3.0 workflow</div></div></header><div class="content">
{{if .Message}}<section class="card" style="margin-bottom:16px"><div class="cardbody green">✓ {{.Message}}</div></section>{{end}}{{if .Error}}<section class="card" style="margin-bottom:16px"><div class="cardbody red">{{.Error}}</div></section>{{end}}
<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Governed workflow</div><div class="cardsub">One configuration artifact continues through the existing FleetAMP controls</div></div></div><div class="cardbody"><div class="upcominggrid"><div class="upcomingitem"><strong>1 · Choose</strong><div class="tiny">Blueprint, owned group, signals and approved destination</div></div><div class="upcomingitem"><strong>2 · Generate</strong><div class="tiny">Explicit Collector YAML with locked safety processors</div></div><div class="upcomingitem"><strong>3 · Validate</strong><div class="tiny">YAML, Collector structure and configured binary checks</div></div><div class="upcomingitem"><strong>4 · Approve & deploy</strong><div class="tiny">Immutable version, reviewer, OpAMP rollout, rollback and drift</div></div></div></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Create configuration from Blueprint</div><div class="cardsub">Group owners can target only groups they own; destination internals remain administrator controlled.</div></div></div><div class="cardbody">{{if and .Groups .Destinations}}<form method="post" action="/blueprints"><input type="hidden" name="action" value="generate"><div class="detailform"><label>Blueprint<select class="select" name="pattern" required>{{range .Patterns}}<option value="{{.ID}}">{{.Name}} — {{.Description}}</option>{{end}}</select></label><label>Target group<select class="select" name="group_id" required>{{range .Groups}}<option value="{{.Group.ID}}">{{.Group.Name}}</option>{{end}}</select></label><label>Destination profile<select class="select" name="destination_id" required>{{range .Destinations}}{{if .Enabled}}<option value="{{.ID}}">{{.Name}} · {{.Environment}}</option>{{end}}{{end}}</select></label><label>Configuration name<input class="input" name="name" required maxlength="120" placeholder="payments-observability"></label><label>Version<input class="input" name="version" required maxlength="60" placeholder="0.3.0-1"></label></div><div style="display:flex;gap:18px;flex-wrap:wrap;margin:16px 0"><label><input type="checkbox" name="signal" value="metrics" checked> Metrics</label><label><input type="checkbox" name="signal" value="traces" checked> Traces</label><label><input type="checkbox" name="signal" value="logs" checked> Logs</label></div><div class="detailform"><label>Assigned reviewer<select class="select" name="assigned_reviewer"><option value="">Required only when requesting approval</option>{{range .Groups}}{{range .Reviewers}}<option value="{{.Username}}">{{.Username}} · {{.Role}}</option>{{end}}{{end}}</select></label><label>Approval validity<select class="select" name="expiry_days"><option value="7">7 days</option><option value="30" selected>30 days</option><option value="60">60 days</option><option value="90">90 days</option></select></label><label style="grid-column:1/-1">Change reason<textarea class="input" name="change_reason" maxlength="1000" rows="3" placeholder="Purpose, expected impact and ticket. Required for approval."></textarea></label></div><div class="detailactions" style="margin-top:14px"><button class="btn" type="submit">Validate and save version</button><button class="btn primary" type="submit" name="submit_for_approval" value="true">Validate, save & request approval</button></div><p class="tiny">Memory limiter, batching and destination details are policy-controlled. Saved versions are immutable. Deployment begins only after approval.</p></form>{{else}}<div class="empty">Create an enabled group and an administrator-approved destination profile before building a configuration.</div>{{end}}</div></section>
{{if .IsAdmin}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Destination profiles</div><div class="cardsub">Admin-only exporter endpoint, authentication and TLS configuration used by Blueprints</div></div></div><div class="cardbody"><form method="post" action="/blueprints"><input type="hidden" name="action" value="create_destination"><div class="detailform"><label>Name<input class="input" name="destination_name" required placeholder="Grafana Cloud"></label><label>Environment<input class="input" name="environment" required placeholder="Production"></label><label>Exporter component ID<input class="input" name="exporter_id" required placeholder="otlphttp/grafana-prod"></label><label style="grid-column:1/-1">Exporter configuration (YAML)<textarea class="input code" name="exporter_config" required rows="6" placeholder="endpoint: https://example.invalid/otlp&#10;headers:&#10;  Authorization: Bearer ${env:OTLP_TOKEN}"></textarea></label></div><button class="btn primary" type="submit">Validate and add destination</button></form>{{if .Destinations}}<div style="overflow:auto;margin-top:18px"><table><thead><tr><th>Destination</th><th>Environment</th><th>Exporter ID</th><th>State</th><th>Action</th></tr></thead><tbody>{{range .Destinations}}<tr><td><strong>{{.Name}}</strong></td><td>{{.Environment}}</td><td class="code">{{.ExporterID}}</td><td>{{if .Enabled}}<span class="badge ok">Enabled</span>{{else}}<span class="badge off">Disabled</span>{{end}}</td><td><form method="post" action="/blueprints" onsubmit="return confirm('Delete this destination profile? Existing immutable configurations are not changed.')"><input type="hidden" name="action" value="delete_destination"><input type="hidden" name="destination_id" value="{{.ID}}"><button class="btn" type="submit">Delete</button></form></td></tr>{{end}}</tbody></table></div>{{end}}</div></section>{{end}}</div></main></div></body></html>`

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
				p := blueprints.NewDestinationProfile(r.FormValue("destination_name"), r.FormValue("environment"), r.FormValue("exporter_id"), r.FormValue("exporter_config"))
				if p.Name == "" || p.Environment == "" || p.ExporterID == "" || p.ExporterConfig == "" {
					http.Error(w, "all destination fields are required", http.StatusUnprocessableEntity)
					return
				}
				var cfg map[string]any
				if err := yaml.Unmarshal([]byte(p.ExporterConfig), &cfg); err != nil || len(cfg) == 0 {
					http.Error(w, "exporter configuration must be a non-empty YAML mapping", http.StatusUnprocessableEntity)
					return
				}
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
			content, err := generateBlueprintYAML(r.FormValue("pattern"), r.Form["signal"], destination)
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
		items := make([]blueprintGroup, 0, len(visible))
		for _, g := range visible {
			reviewers, e := eligibleDeploymentReviewers(r.Context(), auth, g, username, roleNow)
			if e != nil {
				internalServerError(w, e)
				return
			}
			items = append(items, blueprintGroup{Group: g, Reviewers: reviewers})
		}
		if err := blueprintsPage.Execute(w, blueprintView{Page: "blueprints", Groups: items, Destinations: profiles, Patterns: blueprintPatterns, IsAdmin: roleNow == roleAdmin, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error")}); err != nil {
			internalServerError(w, err)
		}
	})
}

func generateBlueprintYAML(pattern string, selected []string, destination *blueprints.DestinationProfile) (string, error) {
	if destination == nil {
		return "", fmt.Errorf("destination profile is required")
	}
	var exporter map[string]any
	if err := yaml.Unmarshal([]byte(destination.ExporterConfig), &exporter); err != nil {
		return "", fmt.Errorf("invalid destination profile: %w", err)
	}
	signalSet := map[string]bool{}
	for _, s := range selected {
		if s == "metrics" || s == "traces" || s == "logs" {
			signalSet[s] = true
		}
	}
	receiverID := "otlp"
	receivers := map[string]any{"otlp": map[string]any{"protocols": map[string]any{"grpc": map[string]any{}, "http": map[string]any{}}}}
	if pattern == "host-observability" {
		receiverID = "hostmetrics"
		receivers = map[string]any{"hostmetrics": map[string]any{"collection_interval": "30s", "scrapers": map[string]any{"cpu": map[string]any{}, "memory": map[string]any{}, "disk": map[string]any{}, "filesystem": map[string]any{}, "network": map[string]any{}}}}
		signalSet = map[string]bool{"metrics": true}
	} else if pattern != "otlp-service" {
		return "", fmt.Errorf("unknown Blueprint pattern")
	}
	if len(signalSet) == 0 {
		return "", fmt.Errorf("select at least one signal")
	}
	pipelines := map[string]any{}
	for _, signal := range []string{"metrics", "traces", "logs"} {
		if signalSet[signal] {
			pipelines[signal] = map[string]any{"receivers": []string{receiverID}, "processors": []string{"memory_limiter", "batch"}, "exporters": []string{destination.ExporterID}}
		}
	}
	doc := map[string]any{
		"receivers":  receivers,
		"processors": map[string]any{"memory_limiter": map[string]any{"check_interval": "1s", "limit_mib": 512, "spike_limit_mib": 128}, "batch": map[string]any{"timeout": "5s"}},
		"exporters":  map[string]any{destination.ExporterID: exporter},
		"extensions": map[string]any{},
		"connectors": map[string]any{},
		"service": map[string]any{
			"extensions": []string{},
			"pipelines":  pipelines,
			"telemetry":  map[string]any{"logs": map[string]any{"level": "info"}},
		},
	}
	b, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
