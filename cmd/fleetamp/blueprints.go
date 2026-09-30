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

const blueprintsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Blueprints</title><style>` + controlPlaneCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Blueprints</div><div class="pagetitle">Blueprints</div><div class="subtitle">Build governed OpenTelemetry configurations without writing YAML, then validate and submit them through approval.</div></div><div class="topactions"><div class="connection"><span class="dot"></span>v0.3.0 workflow</div></div></header><div class="content">
{{if .Message}}<section class="card" style="margin-bottom:16px"><div class="cardbody green">✓ {{.Message}}</div></section>{{end}}{{if .Error}}<section class="card" style="margin-bottom:16px"><div class="cardbody red">{{.Error}}</div></section>{{end}}
<section class="card" style="margin-bottom:16px"><div class="cardhead"><div><div class="cardtitle">Governed workflow</div><div class="cardsub">One configuration artifact continues through the existing FleetAMP controls</div></div></div><div class="cardbody"><div class="upcominggrid"><div class="upcomingitem"><strong>1 · Choose</strong><div class="tiny">Blueprint, owned group, signals and approved destination</div></div><div class="upcomingitem"><strong>2 · Generate</strong><div class="tiny">Explicit Collector YAML with locked safety processors</div></div><div class="upcomingitem"><strong>3 · Validate</strong><div class="tiny">YAML, Collector structure and configured binary checks</div></div><div class="upcomingitem"><strong>4 · Approve & deploy</strong><div class="tiny">Immutable version, reviewer, OpAMP rollout, rollback and drift</div></div></div></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Create configuration from Blueprint</div><div class="cardsub">Group owners can target only groups they own; destination internals remain administrator controlled.</div></div></div><div class="cardbody">{{if and .Groups .Destinations}}<form method="post" action="/blueprints"><input type="hidden" name="action" value="generate"><div class="detailform"><label>Pattern<select class="select" name="pattern" required>{{range .Patterns}}{{if .Enabled}}<option value="{{.ID}}">{{.Name}} · {{.Platform}} — {{.Description}}</option>{{end}}{{end}}</select></label><label>Target group<select class="select" name="group_id" required>{{range .Groups}}<option value="{{.Group.ID}}">{{.Group.Name}}</option>{{end}}</select></label><label>Destination profile<select class="select" name="destination_id" required>{{range .Destinations}}{{if .Enabled}}<option value="{{.ID}}">{{.Name}} · {{.Environment}}</option>{{end}}{{end}}</select></label><label>Configuration name<input class="input" name="name" required maxlength="120" placeholder="payments-observability"></label><label>Version<input class="input" name="version" required maxlength="60" placeholder="0.3.0-1"></label></div><div style="display:flex;gap:18px;flex-wrap:wrap;margin:16px 0"><label><input type="checkbox" name="signal" value="metrics" checked> Metrics</label><label><input type="checkbox" name="signal" value="traces" checked> Traces</label><label><input type="checkbox" name="signal" value="logs" checked> Logs</label></div><div style="margin:16px 0"><strong>Approved component blocks</strong><div class="cardsub">Required and locked blocks are always included. Select any optional blocks needed by this Blueprint.</div><div class="groupgrid" style="margin-top:10px">{{range .Blocks}}{{if .Enabled}}<label class="upcomingitem"><input type="checkbox" name="block_id" value="{{.ID}}" {{if .Required}}checked disabled{{end}}> <strong>{{.Name}}</strong><span class="tiny code">{{.Kind}} · {{.ComponentID}}</span><span class="tiny">{{.Description}}{{if .Locked}} · Admin locked{{end}}</span>{{if .Required}}<input type="hidden" name="block_id" value="{{.ID}}">{{end}}</label>{{end}}{{end}}</div></div><div class="detailform"><label>Assigned reviewer<select class="select" name="assigned_reviewer"><option value="">Required only when requesting approval</option>{{range .Groups}}{{range .Reviewers}}<option value="{{.Username}}">{{.Username}} · {{.Role}}</option>{{end}}{{end}}</select></label><label>Approval validity<select class="select" name="expiry_days"><option value="7">7 days</option><option value="30" selected>30 days</option><option value="60">60 days</option><option value="90">90 days</option></select></label><label style="grid-column:1/-1">Change reason<textarea class="input" name="change_reason" maxlength="1000" rows="3" placeholder="Purpose, expected impact and ticket. Required for approval."></textarea></label></div><div class="detailactions" style="margin-top:14px"><button class="btn" type="submit">Validate and save version</button><button class="btn primary" type="submit" name="submit_for_approval" value="true">Validate, save & request approval</button></div><p class="tiny">Memory limiter, batching and destination details are policy-controlled. Saved versions are immutable. Deployment begins only after approval.</p></form>{{else}}<div class="empty">Create an enabled group and an administrator-approved destination profile before building a configuration.</div>{{end}}</div></section>
{{if .IsAdmin}}<section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Pattern & block catalog</div><div class="cardsub">Admin-managed guided starting points and reusable Collector components</div></div></div><div class="cardbody"><div class="detailgrid"><form class="userforms" method="post" action="/blueprints"><input type="hidden" name="action" value="create_pattern"><strong>Add Pattern</strong><label>Name<input class="input" name="pattern_name" required placeholder="Kubernetes metrics"></label><label>Description<input class="input" name="pattern_description" required></label><label>Platform<select class="select" name="platform"><option>linux</option><option>kubernetes</option><option>application</option><option>any</option></select></label><label>Receiver ID<input class="input code" name="receiver_id" required placeholder="k8s_cluster"></label><label>Signals<select class="select" name="pattern_signals" multiple><option value="metrics">Metrics</option><option value="traces">Traces</option><option value="logs">Logs</option></select></label><label>Receiver configuration (YAML)<textarea class="input code" name="receiver_config" rows="6" required placeholder="collection_interval: 30s"></textarea></label><button class="btn primary" type="submit">Validate and add Pattern</button></form><form class="userforms" method="post" action="/blueprints"><input type="hidden" name="action" value="create_block"><strong>Add component block</strong><label>Name<input class="input" name="block_name" required placeholder="Memory limiter"></label><label>Description<input class="input" name="block_description" required></label><label>Kind<select class="select" name="block_kind"><option value="processors">Processor</option><option value="receivers">Receiver</option><option value="extensions">Extension</option><option value="connectors">Connector</option></select></label><label>Component ID<input class="input code" name="component_id" required placeholder="memory_limiter"></label><label>Signals<select class="select" name="block_signals" multiple><option value="metrics">Metrics</option><option value="traces">Traces</option><option value="logs">Logs</option></select></label><label>Platforms<select class="select" name="block_platforms" multiple><option value="linux">Linux</option><option value="kubernetes">Kubernetes</option><option value="application">Application</option><option value="any">Any</option></select></label><label>Component configuration (YAML)<textarea class="input code" name="block_config" rows="6" required placeholder="check_interval: 1s&#10;limit_mib: 512"></textarea></label><label><input type="checkbox" name="required" value="true"> Required by default</label><label><input type="checkbox" name="locked" value="true"> Locked by admin</label><button class="btn primary" type="submit">Validate and add block</button></form></div>{{if .Patterns}}<div style="overflow:auto;margin-top:18px"><table><thead><tr><th>Pattern</th><th>Platform</th><th>Receiver</th><th>Signals</th><th>Action</th></tr></thead><tbody>{{range .Patterns}}<tr><td><strong>{{.Name}}</strong><div class="tiny">{{.Description}}</div></td><td>{{.Platform}}</td><td class="code">{{.ReceiverID}}</td><td>{{range .Signals}}<span class="chip">{{.}}</span>{{end}}</td><td><form method="post" action="/blueprints"><input type="hidden" name="action" value="delete_pattern"><input type="hidden" name="pattern_id" value="{{.ID}}"><button class="btn" type="submit">Delete</button></form></td></tr>{{end}}</tbody></table></div>{{end}}{{if .Blocks}}<div style="overflow:auto;margin-top:18px"><table><thead><tr><th>Block</th><th>Kind / ID</th><th>Scope</th><th>Policy</th><th>Action</th></tr></thead><tbody>{{range .Blocks}}<tr><td><strong>{{.Name}}</strong><div class="tiny">{{.Description}}</div></td><td class="code">{{.Kind}} / {{.ComponentID}}</td><td>{{range .Platforms}}<span class="chip">{{.}}</span>{{end}} {{range .Signals}}<span class="chip">{{.}}</span>{{end}}</td><td>{{if .Required}}Required {{else}}Optional {{end}}{{if .Locked}}· Locked{{end}}</td><td><form method="post" action="/blueprints"><input type="hidden" name="action" value="delete_block"><input type="hidden" name="block_id" value="{{.ID}}"><button class="btn" type="submit">Delete</button></form></td></tr>{{end}}</tbody></table></div>{{end}}</div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Destination profiles</div><div class="cardsub">Admin-only exporter endpoint, authentication and TLS configuration used by Blueprints</div></div></div><div class="cardbody"><form method="post" action="/blueprints"><input type="hidden" name="action" value="create_destination"><div class="detailform"><label>Name<input class="input" name="destination_name" required placeholder="Grafana Cloud"></label><label>Environment<input class="input" name="environment" required placeholder="Production"></label><label>Exporter component ID<input class="input" name="exporter_id" required placeholder="otlphttp/grafana-prod"></label><label style="grid-column:1/-1">Exporter configuration (YAML)<textarea class="input code" name="exporter_config" required rows="6" placeholder="endpoint: https://example.invalid/otlp&#10;tls:&#10;  insecure: false"></textarea></label></div><button class="btn primary" type="submit">Validate and add destination</button></form>{{if .Destinations}}<div style="overflow:auto;margin-top:18px"><table><thead><tr><th>Destination</th><th>Environment</th><th>Exporter ID</th><th>State</th><th>Action</th></tr></thead><tbody>{{range .Destinations}}<tr><td><strong>{{.Name}}</strong></td><td>{{.Environment}}</td><td class="code">{{.ExporterID}}</td><td>{{if .Enabled}}<span class="badge ok">Enabled</span>{{else}}<span class="badge off">Disabled</span>{{end}}</td><td><form method="post" action="/blueprints" onsubmit="return confirm('Delete this destination profile? Existing immutable configurations are not changed.')"><input type="hidden" name="action" value="delete_destination"><input type="hidden" name="destination_id" value="{{.ID}}"><button class="btn" type="submit">Delete</button></form></td></tr>{{end}}</tbody></table></div>{{end}}</div></section>{{end}}</div></main></div></body></html>`

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
			pattern, err := destinations.GetPattern(r.Context(), strings.TrimSpace(r.FormValue("pattern")))
			if err != nil || !pattern.Enabled { http.Error(w, "enabled Pattern not found", http.StatusNotFound); return }
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

