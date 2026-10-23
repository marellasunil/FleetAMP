package main

import (
	"encoding/json"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type distributionConfigureView struct {
	Page           string
	Distribution   *distributions.Distribution
	Release        *distributions.DistributionRelease
	Groups         []*groups.Group
	Sections       []configs.SectionDefinition
	Previews       []distributionPreviewRow
	Message, Error string
}

func handleDistributionPreviewApproval(w http.ResponseWriter, r *http.Request, distribution *distributions.Distribution, release *distributions.DistributionRelease, visible []*groups.Group, previewStore storage.DistributionPreviewStore, handoffStore storage.DistributionPreviewHandoffStore, _ storage.GroupStore, agentStore storage.ManagedAgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, requestStore storage.GroupDeploymentRequestStore, auth *authManager, notifier *approvalNotifier) {
	preview, err := previewStore.Get(r.Context(), strings.TrimSpace(r.FormValue("preview_id")))
	if err != nil || preview.DistributionID != distribution.ID || preview.DistributionSpecHash != distribution.SpecHash {
		http.Error(w, "the selected immutable preview is unavailable or does not match this distribution", http.StatusConflict)
		return
	}
	if release != nil && (preview.ReleaseID != release.ID || preview.ReleaseEvidenceHash != release.EvidenceHash || preview.ArtifactSHA256 != release.ArtifactSHA256) {
		http.Error(w, "the preview no longer matches the exact released artifact evidence", http.StatusConflict)
		return
	}
	var group *groups.Group
	for _, candidate := range visible {
		if candidate.ID == preview.GroupID {
			group = candidate
			break
		}
	}
	if group == nil {
		http.Error(w, "forbidden: preview group is not assigned to your account", http.StatusForbidden)
		return
	}
	if _, err = handoffStore.GetByPreview(r.Context(), preview.ID); err == nil {
		http.Error(w, "this immutable preview has already been submitted for approval", http.StatusConflict)
		return
	}
	name, version := strings.TrimSpace(r.FormValue("configuration_name")), strings.TrimSpace(r.FormValue("configuration_version"))
	if name == "" || version == "" {
		http.Error(w, "configuration name and version are required", http.StatusUnprocessableEntity)
		return
	}
	configuration := configs.NewGroupConfiguration(group.ID, name, version, preview.ComposedConfig, "text/yaml")
	if configuration.Hash != preview.ContentHash {
		http.Error(w, "configuration content does not match the immutable preview", http.StatusConflict)
		return
	}
	actor := currentUsername(auth, r)
	reviewer, reason := strings.TrimSpace(r.FormValue("assigned_reviewer")), strings.TrimSpace(r.FormValue("change_reason"))
	if reason == "" || len(reason) > 1000 {
		http.Error(w, "change reason is required and must not exceed 1000 characters", http.StatusUnprocessableEntity)
		return
	}
	if err = validateAssignedReviewer(r.Context(), auth, group, actor, reviewer); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	allAgents, err := agentStore.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	members := allAgents[:0]
	for _, agent := range allAgents {
		if groups.MatchesIdentity(group, agent) {
			members = append(members, agent)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
	resolved, eligible, err := previewGroupConfiguration(r.Context(), members, group.Enabled, configuration, assignmentStore)
	if err != nil {
		internalServerError(w, err)
		return
	}
	if eligible == 0 {
		http.Error(w, "no ready target is eligible for this configuration", http.StatusConflict)
		return
	}
	targets := make([]configs.GroupDeploymentTarget, 0, len(resolved))
	for _, item := range resolved {
		targets = append(targets, configs.GroupDeploymentTarget{AgentInstanceUID: item.Agent.InstanceUID, AgentName: item.Agent.Name, Readiness: item.Reason, Eligible: item.Reason == "Ready"})
	}
	request, err := configs.NewGroupDeploymentRequest(group.ID, group.Name, group.Selector, configuration, targets, actor, reviewer, reason)
	if err != nil {
		internalServerError(w, err)
		return
	}
	days, err := approvalExpiryDays(r.FormValue("expiry_days"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	request.ExpiresAt = request.CreatedAt.Add(time.Duration(days) * 24 * time.Hour)
	request.BaseConfigurationID, request.BaseConfigurationHash, err = commonAppliedConfiguration(r.Context(), resolved, assignmentStore)
	if err != nil {
		internalServerError(w, err)
		return
	}
	handoff, err := distributions.NewPreviewHandoff(preview, configuration.ID, configuration.Hash, request.ID, actor, time.Now())
	if err != nil {
		internalServerError(w, err)
		return
	}
	if err = configStore.Put(r.Context(), configuration); err != nil {
		internalServerError(w, err)
		return
	}
	if err = requestStore.Create(r.Context(), request); err != nil {
		internalServerError(w, err)
		return
	}
	if err = handoffStore.Create(r.Context(), handoff); err != nil {
		internalServerError(w, err)
		return
	}
	if notifier != nil {
		notifier.notify(r.Context(), "submitted", request, group)
	}
	http.Redirect(w, r, "/approvals/"+url.PathEscape(request.ID), http.StatusSeeOther)
}

type distributionPreviewRow struct {
	Preview   *distributions.DeploymentPreview
	Handoff   *distributions.PreviewHandoff
	Reviewers []reviewerOption
}

const distributionConfigureHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Configure Distribution · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.configure-layout{display:grid;grid-template-columns:minmax(0,1fr) minmax(320px,.7fr);gap:16px}.section-editor{display:grid;gap:12px}.policy-summary{display:grid;grid-template-columns:1fr 1fr;gap:10px}.handoff-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}@media(max-width:1050px){.configure-layout,.policy-summary,.handoff-grid{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Lifecycle / Distributions / Configure</div><div class="pagetitle">Configure {{.Distribution.Name}}</div><div class="subtitle">Create an immutable, validated configuration preview for an authorized group.</div></div><div class="topactions"><a class="btn" href="/collector-distributions">Back to catalog</a></div></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}<div class="configure-layout"><section class="card"><div class="cardhead"><div><div class="cardtitle">Group configuration overlay</div><div class="cardsub">Only sections permitted by the published distribution are editable.</div></div></div><div class="cardbody"><form method="post"><input type="hidden" name="action" value="preview"><label>Authorized group<select class="select" name="group_id" required><option value="">Select group</option>{{range .Groups}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><div class="policy-summary" style="margin-top:14px"><div class="managed-field"><strong>Organization-managed</strong><div class="chips">{{range .Distribution.LockedSections}}<span class="chip">{{.}}</span>{{end}}</div></div><div class="managed-field"><strong>Group-editable</strong><div class="chips">{{range .Distribution.EditableSections}}<span class="chip">{{.}}</span>{{end}}</div></div></div><div class="section-editor" style="margin-top:14px">{{range .Sections}}<label><strong>{{.Title}}</strong><span class="tiny">{{.Description}}</span><textarea class="input code" name="section_{{.Key}}" rows="7" placeholder="Enter the {{.Title}} YAML fragment"></textarea></label>{{else}}<div class="empty">This distribution exposes no editable configuration sections.</div>{{end}}</div><div class="notice" style="margin-top:14px">FleetAMP composes the submitted sections, validates YAML and pipeline references, and freezes the distribution hash, group and resulting configuration. No deployment occurs.</div><div class="detailactions"><button class="btn primary" type="submit">Validate &amp; save immutable preview</button></div></form></div></section><aside class="card"><div class="cardhead"><div><div class="cardtitle">Distribution evidence</div><div class="cardsub">{{.Distribution.CollectorVersion}} · {{.Distribution.SourceRef}}</div></div></div><div class="cardbody"><div class="tiny code">Specification: {{.Distribution.SpecHash}}</div><div class="tiny code">Admin baseline: {{.Distribution.BaselineHash}}</div><div class="chips"><span class="chip">{{len .Distribution.Components.Receivers}} receivers</span><span class="chip">{{len .Distribution.Components.Processors}} processors</span><span class="chip">{{len .Distribution.Components.Exporters}} exporters</span></div></div></aside></div><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Immutable previews</div><div class="cardsub">Save the exact preview as a version and submit it for four-eyes approval. No deployment occurs.</div></div></div><div class="cardbody">{{range .Previews}}<article class="managed-field" style="margin-bottom:14px"><div style="display:flex;justify-content:space-between;gap:12px"><div><strong>{{.Preview.GroupName}}</strong><div class="tiny code">Preview: {{.Preview.ID}}</div></div>{{if .Handoff}}<span class="badge warn">Pending approval</span>{{else}}<span class="badge ok">Validated</span>{{end}}</div><div class="handoff-grid" style="margin-top:10px"><div class="tiny code">Release: {{.Preview.ReleaseID}}<br>Artifact: {{.Preview.ArtifactSHA256}}</div><div class="tiny code">Configuration content: {{.Preview.ContentHash}}<br>Prepared by {{.Preview.PreparedBy}}</div></div>{{if .Handoff}}<div class="notice" style="margin-top:10px"><strong>Exact evidence submitted</strong><div class="tiny code">Configuration: {{.Handoff.ConfigurationID}} · {{.Handoff.ConfigurationHash}}</div><div class="tiny code">Approval: {{.Handoff.ApprovalRequestID}}</div><div class="tiny code">Handoff: {{.Handoff.EvidenceHash}}</div><a class="btn" style="margin-top:8px" href="/approvals/{{.Handoff.ApprovalRequestID}}">Open approval</a></div>{{else if .Reviewers}}<form method="post" class="detailform" style="margin-top:10px"><input type="hidden" name="action" value="submit_approval"><input type="hidden" name="preview_id" value="{{.Preview.ID}}"><label>Configuration name<input class="input" name="configuration_name" required maxlength="120" placeholder="payments-collector"></label><label>Immutable version<input class="input" name="configuration_version" required maxlength="80" placeholder="1.0.0"></label><label>Assigned reviewer<select class="select" name="assigned_reviewer" required><option value="">Select a different reviewer</option>{{range .Reviewers}}<option value="{{.Username}}">{{.Username}} · {{.Role}}</option>{{end}}</select></label><label>Change reason<textarea class="input" name="change_reason" required maxlength="1000" rows="3" placeholder="Why this released package configuration is needed"></textarea></label><label>Approval validity<select class="select" name="expiry_days"><option value="7">7 days</option><option value="30" selected>30 days</option><option value="60">60 days</option><option value="90">90 days</option></select></label><button class="btn primary" type="submit">Save exact version &amp; request approval</button></form>{{else}}<div class="configerror" style="margin-top:10px">No different Admin or Group Owner is available to review this change.</div>{{end}}</article>{{else}}<div class="empty">No previews have been saved for this distribution.</div>{{end}}</div></section></div></main></div></body></html>`

var distributionConfigurePage = template.Must(template.New("distribution-configure").Parse(strings.Replace(strings.Replace(distributionConfigureHTML, `<div class="tiny code">Admin baseline: {{.Distribution.BaselineHash}}</div>`, `<div class="tiny code">Admin baseline: {{.Distribution.BaselineHash}}</div>{{if .Release}}<div class="tiny code">Release: {{.Release.ID}}</div><div class="tiny code">Artifact: {{.Release.ArtifactURI}}</div><div class="tiny code">Artifact SHA-256: {{.Release.ArtifactSHA256}}</div><div class="tiny code">Release evidence: {{.Release.EvidenceHash}}</div>{{else}}<div class="tiny">Legacy artifact evidence is bound directly from the specification.</div>{{end}}`, 1), `<td class="code">{{.DistributionSpecHash}}</td>`, `<td><div class="tiny code">Spec: {{.DistributionSpecHash}}</div><div class="tiny code">Release: {{.ReleaseID}}</div><div class="tiny code">Artifact: {{.ArtifactSHA256}}</div></td>`, 1)))

func registerDistributionPreviewRoutes(mux *http.ServeMux, distributionStore storage.DistributionStore, releaseStore storage.DistributionReleaseStore, previewStore storage.DistributionPreviewStore, handoffStore storage.DistributionPreviewHandoffStore, groupStore storage.GroupStore, agentStore storage.ManagedAgentStore, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, requestStore storage.GroupDeploymentRequestStore, validator *configs.Validator, auth *authManager, notifier *approvalNotifier) {
	mux.HandleFunc("/collector-distributions/configure", func(w http.ResponseWriter, r *http.Request) {
		distribution, err := distributionStore.Get(r.Context(), strings.TrimSpace(r.URL.Query().Get("id")))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		releases, err := releaseStore.List(r.Context(), 1000)
		if err != nil {
			internalServerError(w, err)
			return
		}
		var release *distributions.DistributionRelease
		for _, item := range releases {
			if item.DistributionID == distribution.ID {
				release = item
				break
			}
		}
		if release == nil && (distribution.ArtifactURI == "" || distribution.ArtifactSHA256 == "") {
			http.Error(w, "this distribution has not been finalized as a released package", http.StatusConflict)
			return
		}
		if strings.TrimSpace(distribution.BaselineConfig) == "" {
			http.Error(w, "this legacy distribution has no Admin baseline; publish a new immutable version before group configuration", http.StatusConflict)
			return
		}
		allGroups, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		visible := groupsVisibleToUser(r.Context(), auth, currentUsername(auth, r), currentRole(auth, r), allGroups)
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", 400)
				return
			}
			if r.FormValue("action") == "submit_approval" {
				handleDistributionPreviewApproval(w, r, distribution, release, visible, previewStore, handoffStore, groupStore, agentStore, configStore, assignmentStore, requestStore, auth, notifier)
				return
			}
			groupID := strings.TrimSpace(r.FormValue("group_id"))
			var selected *groups.Group
			for _, group := range visible {
				if group.ID == groupID {
					selected = group
					break
				}
			}
			if selected == nil {
				http.Error(w, "forbidden: select a group assigned to your account", http.StatusForbidden)
				return
			}
			sections := map[string]string{}
			allowed := map[string]bool{}
			for _, key := range distribution.EditableSections {
				allowed[key] = true
			}
			for _, definition := range configs.ConfigurationSectionDefinitions {
				value := strings.TrimSpace(r.FormValue("section_" + definition.Key))
				if value != "" {
					if !allowed[definition.Key] {
						http.Error(w, "configuration section is organization-managed: "+definition.Title, http.StatusForbidden)
						return
					}
					sections[definition.Key] = value
				}
			}
			content, composeErr := configs.ComposeConfigurationSections(distribution.BaselineConfig, sections)
			if composeErr != nil {
				renderDistributionConfigure(w, r, distribution, release, visible, previewStore, handoffStore, auth, "", composeErr.Error())
				return
			}
			validation := validator.Validate(r.Context(), content)
			if !validation.Valid {
				renderDistributionConfigure(w, r, distribution, release, visible, previewStore, handoffStore, auth, "", validation.Error)
				return
			}
			evidence, _ := json.Marshal(validation)
			preview, previewErr := distributions.NewDeploymentPreview(distribution, release, selected.ID, selected.Name, sections, content, string(evidence), currentUsername(auth, r), time.Now())
			if previewErr == nil {
				previewErr = previewStore.Create(r.Context(), preview)
			}
			if previewErr != nil {
				renderDistributionConfigure(w, r, distribution, release, visible, previewStore, handoffStore, auth, "", previewErr.Error())
				return
			}
			http.Redirect(w, r, "/collector-distributions/configure?id="+url.QueryEscape(distribution.ID)+"&message="+url.QueryEscape("Validated immutable preview saved with exact release evidence. No deployment was performed."), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", 405)
			return
		}
		renderDistributionConfigure(w, r, distribution, release, visible, previewStore, handoffStore, auth, r.URL.Query().Get("message"), "")
	})
}
func renderDistributionConfigure(w http.ResponseWriter, r *http.Request, d *distributions.Distribution, release *distributions.DistributionRelease, visible []*groups.Group, store storage.DistributionPreviewStore, handoffStore storage.DistributionPreviewHandoffStore, auth *authManager, message, pageError string) {
	items, err := store.List(r.Context(), 200)
	if err != nil {
		internalServerError(w, err)
		return
	}
	handoffs, err := handoffStore.List(r.Context(), 500)
	if err != nil {
		internalServerError(w, err)
		return
	}
	byPreview := map[string]*distributions.PreviewHandoff{}
	for _, handoff := range handoffs {
		byPreview[handoff.PreviewID] = handoff
	}
	filtered := make([]distributionPreviewRow, 0)
	for _, item := range items {
		if item.DistributionID == d.ID {
			for _, group := range visible {
				if group.ID == item.GroupID {
					reviewers, reviewerErr := eligibleDeploymentReviewers(r.Context(), auth, group, currentUsername(auth, r), currentRole(auth, r))
					if reviewerErr != nil {
						internalServerError(w, reviewerErr)
						return
					}
					filtered = append(filtered, distributionPreviewRow{Preview: item, Handoff: byPreview[item.ID], Reviewers: reviewers})
					break
				}
			}
		}
	}
	definitions := []configs.SectionDefinition{}
	allowed := map[string]bool{}
	for _, key := range d.EditableSections {
		allowed[key] = true
	}
	for _, definition := range configs.ConfigurationSectionDefinitions {
		if allowed[definition.Key] {
			definitions = append(definitions, definition)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := distributionConfigurePage.Execute(w, distributionConfigureView{Page: "collector-distributions", Distribution: d, Release: release, Groups: visible, Sections: definitions, Previews: filtered, Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}
