package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/configs"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type configurationSectionView struct {
	Key              string
	Title            string
	Description      string
	Content          string
	Editable         bool
	OperatorEditable bool
}

type sectionPolicyPageData struct {
	Page     string
	Policies []sectionPolicyView
	Message  string
	Error    string
}

type sectionPolicyView struct {
	Key              string
	Title            string
	Description      string
	OperatorEditable bool
}

type composeSectionsRequest struct {
	Baseline string            `json:"baseline"`
	Sections map[string]string `json:"sections"`
}

type composeSectionsResponse struct {
	Content    string                   `json:"content,omitempty"`
	Validation configs.ValidationResult `json:"validation"`
}

const sectionEditorCSS = `
.section-tabs{display:flex;gap:7px;flex-wrap:wrap;border-bottom:1px solid var(--line);padding-bottom:10px}
.section-tab{border:1px solid #2b405e;background:#0c1726;color:var(--muted);border-radius:8px;padding:9px 12px;cursor:pointer}
.section-tab.active{color:var(--text);border-color:var(--blue);background:#17294a}
.section-panel{display:none;padding-top:14px}.section-panel.active{display:block}
.section-heading{display:flex;justify-content:space-between;gap:16px;align-items:flex-start;margin-bottom:9px}
.section-editor{width:100%;min-height:330px;resize:vertical;background:#07111e;border:1px solid #29405f;border-radius:8px;padding:13px;color:#d8e5ff;font:12px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace}
.section-editor[readonly]{opacity:.62;cursor:not-allowed;border-style:dashed}
.preview-editor{min-height:360px}
`

const sectionPolicyHTML = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Configuration section policies · FleetAMP</title>
<style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Settings / Configuration</div>
<div class="pagetitle">Configuration section policies</div>
<div class="subtitle">Delegate specific Collector configuration sections to Operators</div></div>
<div class="topactions"><a class="btn" href="/settings/users">Users & roles</a></div></header>
<div class="content">
{{if .Message}}<div class="notice">✓ {{.Message}}</div>{{end}}
{{if .Error}}<div class="configerror" role="alert">{{.Error}}</div>{{end}}
<section class="card"><div class="cardhead"><div><div class="cardtitle">Operator editing policy</div>
<div class="cardsub">Admins can always edit every section. Viewers remain read-only.</div></div></div>
<div style="overflow:auto"><table><thead><tr><th>Section</th><th>Purpose</th><th>Operator access</th></tr></thead><tbody>
{{range .Policies}}<tr><td><strong>{{.Title}}</strong><div class="tiny code">{{.Key}}</div></td>
<td>{{.Description}}</td><td><form method="post" action="/settings/configuration-sections" class="detailform">
<input type="hidden" name="section" value="{{.Key}}">
<select class="select" name="operator_editable"><option value="false" {{if not .OperatorEditable}}selected{{end}}>Read-only</option>
<option value="true" {{if .OperatorEditable}}selected{{end}}>Editable</option></select>
<button class="btn" type="submit">Save</button></form></td></tr>{{end}}
</tbody></table></div></section>
<div class="notice" style="margin-top:16px">Exporters and service wiring are read-only for Operators by default, protecting centrally managed telemetry destinations and routing.</div>
</div></main></div></body></html>`

var sectionPolicyPage = template.Must(template.New("section-policies").Parse(sectionPolicyHTML))

// buildConfigurationSectionViews creates role-aware editor tabs from a
// complete Collector configuration.
func buildConfigurationSectionViews(content string, policies []configs.SectionPolicy, principalRole role) ([]configurationSectionView, error) {
	sections, err := configs.SplitConfigurationSections(content)
	if err != nil {
		return nil, err
	}
	policyByKey := make(map[string]bool, len(policies))
	for _, policy := range policies {
		policyByKey[policy.SectionKey] = policy.OperatorEditable
	}
	result := make([]configurationSectionView, 0, len(configs.ConfigurationSectionDefinitions))
	for _, definition := range configs.ConfigurationSectionDefinitions {
		operatorEditable := policyByKey[definition.Key]
		editable := principalRole == roleAdmin || (principalRole == roleOperator && operatorEditable)
		result = append(result, configurationSectionView{
			Key: definition.Key, Title: definition.Title, Description: definition.Description,
			Content: sections[definition.Key], Editable: editable, OperatorEditable: operatorEditable,
		})
	}
	return result, nil
}

func sectionValuesFromForm(r *http.Request) map[string]string {
	result := make(map[string]string, len(configs.ConfigurationSectionDefinitions))
	for _, definition := range configs.ConfigurationSectionDefinitions {
		result[definition.Key] = r.FormValue("section_" + definition.Key)
	}
	return result
}

func currentRole(auth *authManager, r *http.Request) role {
	if auth != nil {
		if value, ok := auth.sessionRole(r); ok {
			return value
		}
	}
	return roleAdmin
}

func configurationBaseline(ctx context.Context, agentUID string, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, adapter *fleetopamp.Adapter) (string, error) {
	if adapter != nil {
		if effective := adapter.EffectiveConfig(agentUID); strings.TrimSpace(effective) != "" {
			return effective, nil
		}
	}
	assignment, err := latestAssignmentForAgent(ctx, assignmentStore, agentUID)
	if err != nil {
		if err == storage.ErrAssignmentNotFound {
			return "", nil
		}
		return "", err
	}
	configuration, err := configStore.Get(ctx, assignment.ConfigurationID)
	if err != nil {
		return "", err
	}
	return configuration.Content, nil
}

func enforceSectionPolicies(before, after string, policies []configs.SectionPolicy, principalRole role) error {
	if principalRole == roleAdmin {
		return nil
	}
	if principalRole != roleOperator {
		return fmt.Errorf("configuration editing requires Operator or Admin access")
	}
	changed, err := configs.ChangedConfigurationSections(before, after)
	if err != nil {
		return err
	}
	editable := make(map[string]bool, len(policies))
	for _, policy := range policies {
		editable[policy.SectionKey] = policy.OperatorEditable
	}
	var blocked []string
	for _, key := range changed {
		if !editable[key] {
			if definition, ok := configs.SectionDefinitionByKey(key); ok {
				blocked = append(blocked, definition.Title)
			} else {
				blocked = append(blocked, key)
			}
		}
	}
	if len(blocked) > 0 {
		return fmt.Errorf("Admin policy makes these sections read-only: %s", strings.Join(blocked, ", "))
	}
	return nil
}

// registerSectionEditorRoutes exposes the complete-document compose/validate
// endpoint and the Admin policy page.
func registerSectionEditorRoutes(mux *http.ServeMux, policyStore storage.SectionPolicyStore, validator *configs.Validator, auth *authManager) {
	mux.HandleFunc("POST /api/v1/configurations/sections/compose", func(w http.ResponseWriter, r *http.Request) {
		var request composeSectionsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		content, err := configs.ComposeConfigurationSections(request.Baseline, request.Sections)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, composeSectionsResponse{
				Validation: configs.ValidationResult{Error: err.Error()},
			})
			return
		}
		validation := validator.Validate(r.Context(), content)
		status := http.StatusOK
		if !validation.Valid {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, composeSectionsResponse{Content: content, Validation: validation})
	})

	mux.HandleFunc("/settings/configuration-sections", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/configuration-sections" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid section policy request", http.StatusBadRequest)
				return
			}
			key := strings.TrimSpace(r.FormValue("section"))
			editable := strings.EqualFold(r.FormValue("operator_editable"), "true")
			if _, ok := configs.SectionDefinitionByKey(key); !ok {
				http.Error(w, "unsupported configuration section", http.StatusBadRequest)
				return
			}
			if err := policyStore.SetOperatorEditable(r.Context(), key, editable); err != nil {
				internalServerError(w, err)
				return
			}
			actor, _ := auth.sessionUsername(r)
			slog.Info("configuration section policy updated", "component", "config", "event", "section_policy_updated",
				"actor", actor, "section", key, "operator_editable", editable)
			http.Redirect(w, r, "/settings/configuration-sections?message="+url.QueryEscape("Section policy updated."), http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		policies, err := policyStore.List(r.Context())
		if err != nil {
			internalServerError(w, err)
			return
		}
		view := sectionPolicyPageData{Page: "settings", Message: r.URL.Query().Get("message")}
		for _, policy := range policies {
			definition, _ := configs.SectionDefinitionByKey(policy.SectionKey)
			view.Policies = append(view.Policies, sectionPolicyView{
				Key: policy.SectionKey, Title: definition.Title, Description: definition.Description,
				OperatorEditable: policy.OperatorEditable,
			})
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := sectionPolicyPage.Execute(w, view); err != nil {
			slog.Error("render section policy page", "component", "http", "error", err)
		}
	})
}
