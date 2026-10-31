package main

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type configurationTemplateView struct {
	Page           string
	Templates      []*configs.Template
	Selected       *configs.Template
	Groups         []*groups.Group
	Configurations []*configs.Configuration
	CanManage      bool
	IsAdmin        bool
	Message        string
	Error          string
}

const configurationTemplatesHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Configuration Templates · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `
.template-layout{display:grid;grid-template-columns:minmax(340px,.8fr) minmax(0,1.2fr);gap:16px}.template-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.template-card{display:grid;gap:10px;padding:16px}.template-source{display:grid;gap:12px}.template-source>[hidden]{display:none!important}.template-preview{max-height:560px;overflow:auto}@media(max-width:1000px){.template-layout,.template-grid{grid-template-columns:1fr}}
</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Configuration / Builder / Templates</div><div class="pagetitle">Configuration Templates</div><div class="subtitle">Create reusable, validated Collector configurations and instantiate them for authorized groups.</div></div><a class="btn" href="/blueprints">Configuration Builder</a></header><div class="content"><section class="page-description" aria-label="About this page"><div class="page-description-icon" aria-hidden="true">i</div><div><div class="page-description-title">About Configuration Templates</div><p>Save a complete validated Collector configuration manually or derive it from an existing immutable version. Templates are reusable sources, not deployments: using one creates a new group-owned configuration version that must follow the normal preview, approval and deployment workflow.</p></div></section>{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
{{if .CanManage}}<details class="card" style="margin-bottom:16px"><summary class="cardhead"><div><div class="cardtitle">Create template version</div><div class="cardsub">Create manually or copy an existing saved configuration</div></div><span class="btn">New template</span></summary><div class="cardbody"><form method="post" class="template-source" id="template-create"><input type="hidden" name="action" value="create"><div class="detailform"><label>Name<input class="input" name="name" required maxlength="120"></label><label>Version<input class="input" name="version" required maxlength="80" placeholder="1.0.0"></label></div><label>Description<textarea class="input" name="description" rows="2" maxlength="500"></textarea></label><label>Source<select class="select" name="source" id="template-source"><option value="manual">Create manually</option><option value="existing">Existing saved configuration</option></select></label><label id="template-existing" hidden>Saved configuration<select class="select" name="configuration_id"><option value="">Select configuration</option>{{range .Configurations}}<option value="{{.ID}}">{{.Name}} v{{.Version}} · {{.GroupID}}</option>{{end}}</select></label><label id="template-manual">Collector YAML<textarea class="input code" name="content" rows="16" placeholder="receivers:&#10;  otlp:&#10;service:&#10;  pipelines: {}"></textarea></label><label>Visibility<select class="select" name="visibility" id="template-visibility"><option value="selected-groups">Selected groups</option>{{if .IsAdmin}}<option value="organization">Organization-wide (Admin only)</option>{{end}}</select></label><fieldset id="template-groups"><legend>Allowed groups</legend><div class="chips">{{range .Groups}}<label class="chip"><input type="checkbox" name="group_ids" value="{{.ID}}"> {{.Name}}</label>{{end}}</div></fieldset><div class="notice">Template content is immutable. Secret values are not allowed; use existing FleetAMP secret references.</div><button class="btn primary" type="submit">Validate &amp; save template</button></form></div></details>{{end}}
<div class="template-layout"><section class="card"><div class="cardhead"><div><div class="cardtitle">Template library</div><div class="cardsub">Versions visible to your authorized groups</div></div><span class="badge off">{{len .Templates}} versions</span></div><div class="cardbody template-grid">{{range .Templates}}<article class="managed-field template-card"><div><strong>{{.Name}}</strong> <span class="chip">v{{.Version}}</span><div class="tiny">{{.Description}}</div></div><div class="tiny">{{.Visibility}}{{if .GroupIDs}} · {{len .GroupIDs}} group(s){{end}}</div><div class="tiny code">SHA-256: {{.ContentHash}}</div>{{if .SourceConfigurationID}}<div class="tiny code">Source: {{.SourceConfigurationID}}</div>{{end}}<a class="btn" href="/blueprints/templates?template={{.ID}}">Preview &amp; use</a></article>{{else}}<div class="empty">No configuration templates are available.</div>{{end}}</div></section><section class="card"><div class="cardhead"><div><div class="cardtitle">{{if .Selected}}{{.Selected.Name}} v{{.Selected.Version}}{{else}}Template preview{{end}}</div><div class="cardsub">Review exact content before creating a group version</div></div></div><div class="cardbody">{{if .Selected}}<pre class="template-preview">{{.Selected.Content}}</pre><div class="tiny code">Content SHA-256: {{.Selected.ContentHash}}</div>{{if .CanManage}}<form method="post" class="detailform" style="margin-top:16px"><input type="hidden" name="action" value="instantiate"><input type="hidden" name="template_id" value="{{.Selected.ID}}"><label>Target group<select class="select" name="group_id" required><option value="">Select group</option>{{range .Groups}}{{if $.Selected.AllowsGroup .ID}}<option value="{{.ID}}">{{.Name}}</option>{{end}}{{end}}</select></label><label>Configuration name<input class="input" name="configuration_name" value="{{.Selected.Name}}" required maxlength="120"></label><label>New version<input class="input" name="configuration_version" required maxlength="80" placeholder="1.0.0"></label><button class="btn primary" type="submit">Validate &amp; create group version</button></form>{{end}}{{else}}<div class="empty">Select a template to inspect its immutable content and create a group configuration version.</div>{{end}}</div></section></div></div></main></div><script>(function(){var source=document.getElementById('template-source'),manual=document.getElementById('template-manual'),existing=document.getElementById('template-existing'),visibility=document.getElementById('template-visibility'),groups=document.getElementById('template-groups');function syncSource(){if(!source)return;manual.hidden=source.value==='existing';existing.hidden=source.value!=='existing'}function syncVisibility(){if(!visibility)return;groups.hidden=visibility.value==='organization'}if(source){source.addEventListener('change',syncSource);syncSource()}if(visibility){visibility.addEventListener('change',syncVisibility);syncVisibility()}})();</script></body></html>`

var configurationTemplatesPage = template.Must(template.New("configuration-templates").Parse(configurationTemplatesHTML))

func registerConfigurationTemplateRoutes(mux *http.ServeMux, templates storage.ConfigurationTemplateStore, configurations storage.ConfigurationStore, groupStore storage.GroupStore, validator *configs.Validator, auth *authManager) {
	mux.HandleFunc("/blueprints/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blueprints/templates" {
			http.NotFound(w, r)
			return
		}
		username, principalRole := currentUsername(auth, r), currentRole(auth, r)
		allGroups, err := groupStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		visibleGroups := groupsVisibleToUser(r.Context(), auth, username, principalRole, allGroups)
		canManage := auth == nil || principalRole == roleAdmin || principalRole == roleGroupOwner
		if r.Method == http.MethodPost {
			if !canManage {
				http.Error(w, "template management requires Admin or Group Owner access", http.StatusForbidden)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			switch r.FormValue("action") {
			case "create":
				createConfigurationTemplate(w, r, templates, configurations, visibleGroups, validator, auth, principalRole)
			case "instantiate":
				instantiateConfigurationTemplate(w, r, templates, configurations, visibleGroups, validator, auth)
			default:
				http.Error(w, "unsupported template action", http.StatusBadRequest)
			}
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		renderConfigurationTemplates(w, r, templates, configurations, visibleGroups, canManage, auth == nil || principalRole == roleAdmin, r.URL.Query().Get("message"), "")
	})
}

func createConfigurationTemplate(w http.ResponseWriter, r *http.Request, templates storage.ConfigurationTemplateStore, configurations storage.ConfigurationStore, visibleGroups []*groups.Group, validator *configs.Validator, auth *authManager, principalRole role) {
	content, sourceID := strings.TrimSpace(r.FormValue("content")), ""
	if r.FormValue("source") == "existing" {
		source, err := configurations.Get(r.Context(), strings.TrimSpace(r.FormValue("configuration_id")))
		if err != nil || !configurationVisibleToGroups(source, visibleGroups) {
			http.Error(w, "saved configuration is not available", http.StatusForbidden)
			return
		}
		content, sourceID = source.Content, source.ID
	}
	validation := validator.Validate(r.Context(), content)
	if !validation.Valid {
		renderConfigurationTemplates(w, r, templates, configurations, visibleGroups, true, auth == nil || principalRole == roleAdmin, "", "Template validation failed: "+validation.Error)
		return
	}
	visibility := configs.TemplateVisibility(r.FormValue("visibility"))
	if visibility == configs.TemplateOrganization && principalRole != roleAdmin && auth != nil {
		http.Error(w, "only an administrator can publish organization-wide templates", http.StatusForbidden)
		return
	}
	groupIDs := r.Form["group_ids"]
	if visibility == configs.TemplateSelected && !templateGroupsVisible(groupIDs, visibleGroups) {
		http.Error(w, "one or more selected groups are outside your scope", http.StatusForbidden)
		return
	}
	item, err := configs.NewTemplate(r.FormValue("name"), r.FormValue("version"), r.FormValue("description"), content, visibility, groupIDs, sourceID, currentUsername(auth, r), time.Now())
	if err == nil {
		err = templates.Create(r.Context(), item)
	}
	if err != nil {
		renderConfigurationTemplates(w, r, templates, configurations, visibleGroups, true, auth == nil || principalRole == roleAdmin, "", err.Error())
		return
	}
	http.Redirect(w, r, "/blueprints/templates?template="+url.QueryEscape(item.ID)+"&message="+url.QueryEscape("Immutable template version saved."), http.StatusSeeOther)
}

func instantiateConfigurationTemplate(w http.ResponseWriter, r *http.Request, templates storage.ConfigurationTemplateStore, configurations storage.ConfigurationStore, visibleGroups []*groups.Group, validator *configs.Validator, auth *authManager) {
	item, err := templates.Get(r.Context(), strings.TrimSpace(r.FormValue("template_id")))
	if err != nil {
		http.Error(w, "configuration template not found", http.StatusNotFound)
		return
	}
	groupID := strings.TrimSpace(r.FormValue("group_id"))
	if !groupIDVisible(groupID, visibleGroups) || !item.AllowsGroup(groupID) {
		http.Error(w, "template is not available to the selected group", http.StatusForbidden)
		return
	}
	validation := validator.Validate(r.Context(), item.Content)
	if !validation.Valid {
		http.Error(w, "template no longer passes configuration validation", http.StatusUnprocessableEntity)
		return
	}
	configuration := configs.NewGroupConfiguration(groupID, strings.TrimSpace(r.FormValue("configuration_name")), strings.TrimSpace(r.FormValue("configuration_version")), item.Content, "text/yaml")
	if configuration.Name == "" || configuration.Version == "" {
		http.Error(w, "configuration name and version are required", http.StatusUnprocessableEntity)
		return
	}
	if err := configurations.Put(r.Context(), configuration); err != nil {
		internalServerError(w, err)
		return
	}
	http.Redirect(w, r, "/groups/"+url.PathEscape(groupID)+"?configuration_saved="+url.QueryEscape(configuration.ID), http.StatusSeeOther)
}

func renderConfigurationTemplates(w http.ResponseWriter, r *http.Request, templates storage.ConfigurationTemplateStore, configurations storage.ConfigurationStore, visibleGroups []*groups.Group, canManage, isAdmin bool, message, pageError string) {
	allTemplates, err := templates.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	visibleTemplates := make([]*configs.Template, 0, len(allTemplates))
	for _, item := range allTemplates {
		if item.Visibility == configs.TemplateOrganization || templateAllowsAnyVisibleGroup(item, visibleGroups) {
			visibleTemplates = append(visibleTemplates, item)
		}
	}
	allConfigurations, err := configurations.List(r.Context())
	if err != nil {
		internalServerError(w, err)
		return
	}
	visibleConfigurations := make([]*configs.Configuration, 0, len(allConfigurations))
	for _, configuration := range allConfigurations {
		if configurationVisibleToGroups(configuration, visibleGroups) {
			visibleConfigurations = append(visibleConfigurations, configuration)
		}
	}
	var selected *configs.Template
	if id := strings.TrimSpace(r.URL.Query().Get("template")); id != "" {
		candidate, getErr := templates.Get(r.Context(), id)
		if getErr == nil && (candidate.Visibility == configs.TemplateOrganization || templateAllowsAnyVisibleGroup(candidate, visibleGroups)) {
			selected = candidate
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := configurationTemplatesPage.Execute(w, configurationTemplateView{Page: "blueprints", Templates: visibleTemplates, Selected: selected, Groups: visibleGroups, Configurations: visibleConfigurations, CanManage: canManage, IsAdmin: isAdmin, Message: message, Error: pageError}); err != nil {
		internalServerError(w, err)
	}
}

func templateGroupsVisible(groupIDs []string, visible []*groups.Group) bool {
	if len(groupIDs) == 0 {
		return false
	}
	for _, id := range groupIDs {
		if !groupIDVisible(id, visible) {
			return false
		}
	}
	return true
}

func groupIDVisible(id string, visible []*groups.Group) bool {
	for _, group := range visible {
		if group.ID == id {
			return true
		}
	}
	return false
}

func templateAllowsAnyVisibleGroup(item *configs.Template, visible []*groups.Group) bool {
	for _, group := range visible {
		if item.AllowsGroup(group.ID) {
			return true
		}
	}
	return false
}
