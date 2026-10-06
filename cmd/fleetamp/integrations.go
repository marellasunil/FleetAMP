package main

import (
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"html/template"
	"net/http"
	"strings"
)

type integrationsView struct {
	Page           string
	Providers      []integrations.Provider
	Connections    []*integrations.Connection
	Groups         []*groups.Group
	Message, Error string
}

const integrationsHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Integrations · FleetAMP</title><style>` + controlPlaneCSS + detailCSS + `.integration-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}@media(max-width:1050px){.integration-grid{grid-template-columns:1fr}}</style></head><body><div class="shell">` + sideNav + `<main class="main"><header class="top"><div><div class="crumb">FleetAMP / Administration / Integrations</div><div class="pagetitle">Git Integrations</div><div class="subtitle">Scope existing repositories for governed delivery.</div></div><span class="badge off">No provider API calls</span></header><div class="content">{{if .Message}}<div class="notice">{{.Message}}</div>{{end}}{{if .Error}}<div class="configerror">{{.Error}}</div>{{end}}<section class="card"><div class="cardhead"><div><div class="cardtitle">Register repository connection</div><div class="cardsub">Stores scope and a secret reference only; never a raw token.</div></div></div><div class="cardbody"><form method="post"><div class="form-grid"><label class="field">Name<input class="input" name="name" required></label><label class="field">Provider<select class="select" name="provider">{{range .Providers}}<option value="{{.ID}}">{{.Name}}</option>{{end}}</select></label><label class="field">Base URL<input class="input" name="base_url" placeholder="Optional for provider cloud"></label><label class="field">Organization / group<input class="input" name="organization" required></label><label class="field">Azure DevOps project<input class="input" name="project"></label><label class="field">Existing repository<input class="input" name="repository" required></label><label class="field">Branch<input class="input" name="branch" value="main" required></label><label class="field">Allowed root<input class="input" name="allowed_root" value="fleetamp/groups" required></label><label class="field">Mode<select class="select" name="mode"><option value="fleetamp-pull-request">FleetAMP pull request</option><option value="git-managed">Git managed</option><option value="observe-only">Observe only</option><option value="fleetamp-direct-commit">Direct commit</option><option value="fleetamp-primary-sync">FleetAMP primary sync</option></select></label><label class="field">Credential reference<input class="input" name="credential_ref" required placeholder="secret://github/production"></label><fieldset class="field full"><legend>Allowed FleetAMP groups</legend>{{range .Groups}}<label><input type="checkbox" name="group_ids" value="{{.ID}}"> {{.Name}}</label>{{else}}<span class="tiny">Create a group first.</span>{{end}}</fieldset><label class="field"><input type="checkbox" name="enabled" value="true" checked> Enabled</label></div><div class="notice">Repository/pipeline creation, connection tests and Git writes remain disabled.</div><div class="detailactions"><button class="btn primary">Register connection scope</button></div></form></div></section><section class="card" style="margin-top:16px"><div class="cardhead"><div><div class="cardtitle">Registered connections</div><div class="cardsub">Immutable repository security boundaries.</div></div></div>{{if .Connections}}<div style="overflow:auto"><table><thead><tr><th>Name</th><th>Provider / repository</th><th>Branch / allowed root</th><th>Mode</th><th>Groups</th><th>Credential</th><th>Status</th></tr></thead><tbody>{{range .Connections}}<tr><td><strong>{{.Name}}</strong><div class="tiny code">{{.ID}}</div></td><td>{{.Provider}}<div class="tiny">{{.Organization}}{{if .Project}}/{{.Project}}{{end}}/{{.Repository}}</div></td><td>{{.Branch}}<div class="tiny code">{{.AllowedRoot}}</div></td><td>{{.Mode}}</td><td>{{len .GroupIDs}}</td><td class="code">{{.CredentialRef}}</td><td><span class="badge {{if .Enabled}}ok{{else}}off{{end}}">{{if .Enabled}}Enabled{{else}}Disabled{{end}}</span></td></tr>{{end}}</tbody></table></div>{{else}}<div class="empty">No connections registered.</div>{{end}}</section><section class="card" style="margin-top:16px"><div class="cardbody integration-grid">{{range .Providers}}<article class="component-item"><strong>{{.Name}}</strong><div class="tiny">{{.Description}}</div></article>{{end}}</div></section></div></main></div></body></html>`

const integrationFormLayoutCSS = `<style>
.cardbody>form>.form-grid{display:grid!important;grid-template-columns:1fr!important;gap:0!important}
.cardbody>form>.form-grid>.field{display:grid!important;grid-template-columns:minmax(170px,.65fr) minmax(300px,1.35fr) minmax(260px,1fr)!important;grid-template-rows:auto!important;align-items:center;column-gap:18px;row-gap:6px;padding:13px 0;border-bottom:1px solid var(--line)}
.cardbody>form>.form-grid>.field.full{grid-column:auto!important}
.cardbody>form>.form-grid>.field::after{color:var(--muted);font-size:13px;font-weight:400;line-height:1.45;align-self:center}
.cardbody>form>.form-grid>.field:nth-child(1)::after{content:"A clear FleetAMP name for this repository connection."}
.cardbody>form>.form-grid>.field:nth-child(2)::after{content:"Choose GitHub, GitLab or Azure DevOps."}
.cardbody>form>.form-grid>.field:nth-child(3)::after{content:"Leave empty for provider cloud; set it for a self-hosted service."}
.cardbody>form>.form-grid>.field:nth-child(4)::after{content:"The existing organization, owner or top-level group."}
.cardbody>form>.form-grid>.field:nth-child(5)::after{content:"Required only when the provider is Azure DevOps."}
.cardbody>form>.form-grid>.field:nth-child(6)::after{content:"FleetAMP uses an existing repository and never creates one."}
.cardbody>form>.form-grid>.field:nth-child(7)::after{content:"The governed branch used for previews and approved changes."}
.cardbody>form>.form-grid>.field:nth-child(8)::after{content:"A relative directory that generated files cannot escape."}
.cardbody>form>.form-grid>.field:nth-child(9)::after{content:"Select the governance workflow; execution remains disabled."}
.cardbody>form>.form-grid>.field:nth-child(10)::after{content:"Use a secret reference only. Never paste a token or password."}
.cardbody>form>.form-grid>fieldset.field{display:block!important;padding:13px 0;margin:0;border:0;border-bottom:1px solid var(--line)}
.cardbody>form>.form-grid>fieldset.field legend{font-weight:650;margin-bottom:10px}
.cardbody>form>.form-grid>fieldset.field label{display:inline-flex;align-items:center;gap:7px;padding:8px 11px;margin:0 7px 7px 0;border:1px solid var(--line);border-radius:8px;background:#0a1626}
.cardbody>form>.form-grid>.field:last-child{display:flex!important;align-items:center;gap:8px;border-bottom:0}
@media(max-width:950px){.cardbody>form>.form-grid>.field{grid-template-columns:minmax(150px,.55fr) minmax(260px,1fr)!important}.cardbody>form>.form-grid>.field::after{grid-column:2}}
@media(max-width:650px){.cardbody>form>.form-grid>.field{grid-template-columns:1fr!important}.cardbody>form>.form-grid>.field::after{grid-column:auto}}

/* Progressive disclosure: Azure DevOps is the only provider with a project field. */
.cardbody>form>.form-grid>.field:nth-child(5){display:none!important}
.cardbody>form>.form-grid:has(select[name="provider"] option[value="azure-devops"]:checked)>.field:nth-child(5){display:grid!important}

</style>`

var integrationsPage = template.Must(template.New("integrations").Parse(strings.Replace(integrationsHTML, "</head>", integrationFormLayoutCSS+"</head>", 1)))

func registerIntegrationRoutes(mux *http.ServeMux, catalog *integrations.Catalog, connections storage.IntegrationConnectionStore, groupStore storage.GroupStore, auth *authManager) {
	mux.HandleFunc("/settings/integrations", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/integrations" {
			http.NotFound(w, r)
			return
		}
		errorMessage := ""
		if r.Method == http.MethodPost {
			if currentRole(auth, r) != roleAdmin {
				http.Error(w, "administrator access required", http.StatusForbidden)
				return
			}
			if e := r.ParseForm(); e != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			creator := currentUsername(auth, r)
			if creator == "" && auth == nil {
				creator = "test-admin"
			}
			v, e := integrations.NewConnection(integrations.Connection{Name: r.FormValue("name"), Provider: integrations.ProviderID(r.FormValue("provider")), BaseURL: r.FormValue("base_url"), Organization: r.FormValue("organization"), Project: r.FormValue("project"), Repository: r.FormValue("repository"), Branch: r.FormValue("branch"), AllowedRoot: r.FormValue("allowed_root"), Mode: r.FormValue("mode"), CredentialRef: r.FormValue("credential_ref"), GroupIDs: r.Form["group_ids"], Enabled: r.FormValue("enabled") == "true"}, creator)
			if e != nil {
				errorMessage = e.Error()
			} else if e = connections.Create(r.Context(), v); e != nil {
				errorMessage = "Connection name already exists."
			} else {
				http.Redirect(w, r, "/settings/integrations?created="+v.ID, http.StatusSeeOther)
				return
			}
		} else if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rows, e := connections.List(r.Context(), 200)
		if e != nil {
			internalServerError(w, e)
			return
		}
		groupRows, e := groupStore.List(r.Context())
		if e != nil {
			internalServerError(w, e)
			return
		}
		message := ""
		if strings.TrimSpace(r.URL.Query().Get("created")) != "" {
			message = "Connection scope registered. No provider API was called and no Git content changed."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e := integrationsPage.Execute(w, integrationsView{Page: "settings-integrations", Providers: catalog.List(), Connections: rows, Groups: groupRows, Message: message, Error: errorMessage}); e != nil {
			internalServerError(w, e)
		}
	})
}
