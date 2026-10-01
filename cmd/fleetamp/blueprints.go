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
	Starters       []*blueprints.Starter
	Selected       *blueprints.Starter
	IsAdmin        bool
	Message, Error string
}

const blueprintsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>FleetAMP Blueprints</title><style>` + controlPlaneCSS + `
.blueprint-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.blueprint-card{display:flex;flex-direction:column;padding:18px;min-height:250px}.blueprint-card h3{font-size:17px;margin:8px 0}.blueprint-card p{color:var(--muted);margin:0 0 16px}.blueprint-card .btn{margin-top:auto;text-align:center}.chips{display:flex;gap:7px;flex-wrap:wrap;margin:10px 0 16px}.chip{border:1px solid #304866;background:#101e31;border-radius:999px;color:#9db1d0;font-size:11px;padding:4px 8px}.blueprint-detail{display:grid;grid-template-columns:minmax(0,1fr) minmax(320px,.72fr);gap:18px}.summary-box{border:1px solid #263a55;background:#0a1626;border-radius:10px;padding:16px;margin-top:14px}.summary-box h4{margin:0 0 9px}.summary-box ul{margin:0;padding-left:20px;color:var(--muted)}.flowline{padding:12px 14px;border:1px solid #34517a;border-radius:9px;background:#10213b;color:#c9d8f2;font-weight:650;margin:14px 0}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:13px}.field{display:flex;flex-direction:column;gap:6px;color:#9fb0c8;font-size:12px}.field.full{grid-column:1/-1}.field .input,.field .select{width:100%;min-width:0}.form-actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:16px}.notice{border:1px solid #2f735e;background:#10372d;color:#77dfba;border-radius:8px;padding:10px 12px;margin-bottom:16px}.notice.error{border-color:#7a3a42;background:#3b1b22;color:#ff9aa5}@media(max-width:1100px){.blueprint-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.blueprint-detail{grid-template-columns:1fr}}@media(max-width:720px){.blueprint-grid,.form-grid{grid-template-columns:1fr}.field.full{grid-column:auto}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Instrumentation / Blueprints</div><div class="pagetitle">Blueprints</div><div class="subtitle">Start with a proven observability design, then validate, version and submit it through governance.</div></div><div class="topactions"><a class="btn" href="/instrumentation">Open Guides</a></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="notice error">{{.Error}}</div>{{end}}
{{if .Selected}}<div class="sectionhead"><div><div class="eyebrow">{{.Selected.Category}} Blueprint</div><div class="sectiontitle">{{.Selected.Name}}</div><div class="subtitle">{{.Selected.Description}}</div></div><a class="btn" href="/blueprints">← All Blueprints</a></div><div class="blueprint-detail"><section class="card"><div class="cardhead"><div><div class="cardtitle">Design summary</div><div class="cardsub">Review the recommended flow before creating a configuration version.</div></div></div><div class="cardbody"><div class="flowline">{{.Selected.Topology}}</div><div class="chips">{{range .Selected.Signals}}<span class="chip">{{.}}</span>{{end}}<span class="chip">{{.Selected.Platform}}</span><span class="chip">{{.Selected.Method}}</span></div><div class="summary-box"><h4>What this provides</h4><ul>{{range .Selected.Outcomes}}<li>{{.}}</li>{{end}}</ul></div><div class="summary-box"><h4>Before you deploy</h4><ul>{{range .Selected.Requirements}}<li>{{.}}</li>{{end}}</ul></div><p class="tiny">FleetAMP adds the governed memory limiter and batch processor. The selected Destination Profile supplies the approved exporter and protected endpoint settings.</p></div></section>
<section class="card"><div class="cardhead"><div><div class="cardtitle">Create configuration version</div><div class="cardsub">Generation is validated before anything is saved or sent for approval.</div></div></div><div class="cardbody">{{if not .Destinations}}<div class="notice error">An administrator must create an enabled Destination Profile before this Blueprint can be used.</div>{{end}}<form method="post" action="/blueprints"><input type="hidden" name="action" value="generate"><input type="hidden" name="goal" value="{{.Selected.Goal}}"><input type="hidden" name="platform" value="{{.Selected.Platform}}"><input type="hidden" name="technology" value="{{.Selected.Technology}}"><input type="hidden" name="instrumentation_method" value="{{.Selected.Method}}"><input type="hidden" name="pattern" value="{{.Selected.Pattern.ID}}">{{range .Selected.Signals}}<input type="hidden" name="signal" value="{{.}}">{{end}}<div class="form-grid"><label class="field full">Target group<select class="select" name="group_id" id="blueprint-group" required><option value="">Select a group</option>{{range .Groups}}<option value="{{.Group.ID}}">{{.Group.Name}}</option>{{end}}</select></label><label class="field full">Destination Profile<select class="select" name="destination_id" required><option value="">Select a destination</option>{{range .Destinations}}{{if .Enabled}}<option value="{{.ID}}">{{.Name}} · {{.Environment}}</option>{{end}}{{end}}</select></label><label class="field">Configuration name<input class="input" name="name" value="{{.Selected.Name}}" required></label><label class="field">Version<input class="input" name="version" placeholder="for example 1.0.0" required></label><label class="field full">Assigned reviewer<select class="select" name="assigned_reviewer" id="blueprint-reviewer"><option value="">Select when requesting approval</option>{{range .Groups}}{{$group := .Group.ID}}{{range .Reviewers}}<option value="{{.Username}}" data-group="{{$group}}">{{.DisplayName}}</option>{{end}}{{end}}</select></label><label class="field full">Change reason<textarea class="input" name="change_reason" rows="3" placeholder="Why is this observability change needed?"></textarea></label><label class="field">Approval expiry<select class="select" name="expiry_days"><option value="30">30 days</option><option value="60">60 days</option><option value="90">90 days</option></select></label></div><div class="form-actions"><button class="btn" type="submit" name="submit_for_approval" value="false" {{if not .Destinations}}disabled{{end}}>Validate & save version</button><button class="btn primary" type="submit" name="submit_for_approval" value="true" {{if not .Destinations}}disabled{{end}}>Validate, save & request approval</button></div></form></div></section></div>
{{else}}<div class="sectionhead"><div><div class="eyebrow">Starter catalog</div><div class="sectiontitle">Choose what you want to observe</div><div class="subtitle">Common Blueprints are starting points, not fixed deployment templates. Review organization-specific values before approval.</div></div></div><div class="blueprint-grid">{{range .Starters}}<article class="card blueprint-card"><div class="eyebrow">{{.Category}}</div><h3>{{.Name}}</h3><p>{{.Description}}</p><div class="chips">{{range .Signals}}<span class="chip">{{.}}</span>{{end}}<span class="chip">{{.Platform}}</span></div><a class="btn primary" href="/blueprints?starter={{.ID}}">Use Blueprint</a></article>{{end}}</div>{{end}}</div></main></div><script>(function(){var group=document.getElementById('blueprint-group'),reviewer=document.getElementById('blueprint-reviewer');if(!group||!reviewer)return;function sync(){var selected=group.value;Array.from(reviewer.options).forEach(function(option){if(!option.dataset.group)return;option.hidden=option.dataset.group!==selected;option.disabled=option.dataset.group!==selected});reviewer.value=''}group.addEventListener('change',sync);sync()})();</script></body></html>`

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
		starters := blueprints.CommonStarters()
		var selected *blueprints.Starter
		selectedID := strings.TrimSpace(r.URL.Query().Get("starter"))
		for _, starter := range starters {
			if starter.ID == selectedID { selected = starter; break }
		}
		if selectedID != "" && selected == nil { http.NotFound(w, r); return }
		if err := blueprintsPage.Execute(w, blueprintView{Page: "blueprints", Groups: items, Destinations: profiles, Patterns: patterns, Blocks: blocks, Starters: starters, Selected: selected, IsAdmin: roleNow == roleAdmin, Message: r.URL.Query().Get("message"), Error: r.URL.Query().Get("error")}); err != nil {
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
	existingPatterns := make(map[string]bool, len(patterns))
	for _, pattern := range patterns { existingPatterns[pattern.ID] = true }
	for _, starter := range blueprints.CommonStarters() {
		if !existingPatterns[starter.Pattern.ID] { if err := store.CreatePattern(ctx, starter.Pattern); err != nil { return err } }
	}
	blocks, err := store.ListBlocks(ctx)
	if err != nil { return err }
	existingBlocks := make(map[string]bool, len(blocks))
	for _, block := range blocks { existingBlocks[block.ID] = true }
	for _, block := range blueprints.CommonSafetyBlocks() {
		if !existingBlocks[block.ID] { if err := store.CreateBlock(ctx, block); err != nil { return err } }
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
		"deployment": {"kubernetes": {"kubernetes"}, "infrastructure": {"kubernetes"}, "custom": {"kubernetes"}},
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
