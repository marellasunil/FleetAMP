package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/configs"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"gopkg.in/yaml.v3"
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
	GroupID  string            `json:"group_id,omitempty"`
}

type composeSectionsResponse struct {
	Content    string                    `json:"content,omitempty"`
	Validation configs.ValidationResult  `json:"validation"`
	Pipeline   *configs.PipelineModel    `json:"pipeline,omitempty"`
	Components *configs.ComponentCatalog `json:"components,omitempty"`
}

type referenceValidationResponse struct {
	Valid       bool                      `json:"valid"`
	Error       string                    `json:"error,omitempty"`
	Warnings    []string                  `json:"warnings,omitempty"`
	Diagnostics []configurationDiagnostic `json:"diagnostics,omitempty"`
}

type configurationDiagnostic struct {
	Section      string `json:"section"`
	SectionTitle string `json:"section_title"`
	Line         int    `json:"line"`
	Column       int    `json:"column"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
}

var (
	yamlLinePattern           = regexp.MustCompile(`line ([0-9]+)(?:: column ([0-9]+))?`)
	undefinedReferencePattern = regexp.MustCompile(`(?:pipeline "[^"]+"|service) references undefined (receiver|processor|exporter|extension) "([^"]+)"`)
	pipelineNamePattern       = regexp.MustCompile(`pipeline "([^"]+)"`)
)

func referenceDiagnostics(sections map[string]string, validationErr error) []configurationDiagnostic {
	if validationErr == nil {
		return nil
	}
	message := validationErr.Error()
	for _, definition := range configs.ConfigurationSectionDefinitions {
		var document yaml.Node
		if err := yaml.Unmarshal([]byte(sections[definition.Key]), &document); err != nil {
			line, column := yamlErrorPosition(err.Error())
			return []configurationDiagnostic{newConfigurationDiagnostic(definition.Key, line, column, message)}
		}
	}
	if match := undefinedReferencePattern.FindStringSubmatch(message); len(match) == 3 {
		section := configs.SectionServicePipelines
		if match[1] == "extension" {
			section = configs.SectionServiceExtensions
		}
		line, column := yamlScalarPosition(sections[section], match[2])
		return []configurationDiagnostic{newConfigurationDiagnostic(section, line, column, message)}
	}
	if match := pipelineNamePattern.FindStringSubmatch(message); len(match) == 2 {
		line, column := yamlScalarPosition(sections[configs.SectionServicePipelines], match[1])
		return []configurationDiagnostic{newConfigurationDiagnostic(configs.SectionServicePipelines, line, column, message)}
	}
	for _, definition := range configs.ConfigurationSectionDefinitions {
		if strings.HasPrefix(message, definition.Title+":") {
			line, column := yamlErrorPosition(message)
			return []configurationDiagnostic{newConfigurationDiagnostic(definition.Key, line, column, message)}
		}
	}
	return nil
}

func newConfigurationDiagnostic(section string, line, column int, message string) configurationDiagnostic {
	title := section
	if definition, ok := configs.SectionDefinitionByKey(section); ok {
		title = definition.Title
	}
	if line < 1 {
		line = 1
	}
	if column < 1 {
		column = 1
	}
	return configurationDiagnostic{Section: section, SectionTitle: title, Line: line, Column: column, Severity: "error", Message: message}
}

func yamlErrorPosition(message string) (int, int) {
	match := yamlLinePattern.FindStringSubmatch(message)
	if len(match) < 2 {
		return 1, 1
	}
	line, _ := strconv.Atoi(match[1])
	column := 1
	if len(match) > 2 && match[2] != "" {
		column, _ = strconv.Atoi(match[2])
	}
	return line, column
}

func yamlScalarPosition(content, value string) (int, int) {
	var document yaml.Node
	if yaml.Unmarshal([]byte(content), &document) == nil {
		var visit func(*yaml.Node) (int, int, bool)
		visit = func(node *yaml.Node) (int, int, bool) {
			if node.Kind == yaml.ScalarNode && node.Value == value {
				return node.Line, node.Column, true
			}
			for _, child := range node.Content {
				if line, column, ok := visit(child); ok {
					return line, column, true
				}
			}
			return 0, 0, false
		}
		if line, column, ok := visit(&document); ok {
			return line, column
		}
	}
	return 1, 1
}

const sectionEditorCSS = `
.section-tabs{display:flex;gap:6px;overflow-x:auto;padding:6px;background:#091523;border:1px solid #243750;border-radius:12px;scrollbar-width:thin}
.section-tab{position:relative;flex:0 0 auto;border:0;background:transparent;color:#91a2bb;border-radius:8px;padding:10px 13px;cursor:pointer;font-weight:650;transition:background .16s ease,color .16s ease,box-shadow .16s ease}
.section-tab:hover{color:var(--text);background:#12233a}
.section-tab.active{color:#f4f7ff;background:linear-gradient(135deg,#294b91,#3e61ad);box-shadow:0 5px 16px #050b1566}
.section-tab.active:after{content:"";position:absolute;left:30%;right:30%;bottom:3px;height:2px;border-radius:9px;background:#9db4ff}
.section-panel{display:none;padding-top:14px}.section-panel.active{display:block}
.section-heading{display:flex;justify-content:space-between;gap:16px;align-items:flex-start;margin-bottom:9px}
.section-editor{width:100%;min-height:330px;resize:vertical;background:#07111e;border:1px solid #29405f;border-radius:8px;padding:13px;color:#d8e5ff;font:12px/1.55 ui-monospace,SFMono-Regular,Menlo,monospace}
.section-editor[readonly]{opacity:.62;cursor:not-allowed;border-style:dashed}
.preview-editor{min-height:360px}
.yaml-completion{margin-top:7px;border:1px solid #365071;border-radius:9px;background:#091523;box-shadow:0 12px 28px #02071399;overflow:hidden}.yaml-completion[hidden]{display:none}.yaml-completion-head{display:flex;align-items:center;justify-content:space-between;padding:7px 10px;border-bottom:1px solid #243750;color:var(--muted);font-size:11px}.yaml-completion-list{display:grid;max-height:220px;overflow-y:auto}.yaml-completion-item{display:grid;grid-template-columns:minmax(120px,.7fr) minmax(180px,1.3fr);gap:12px;width:100%;border:0;border-bottom:1px solid #182a42;background:transparent;padding:8px 10px;color:var(--text);text-align:left;cursor:pointer}.yaml-completion-item:last-child{border-bottom:0}.yaml-completion-item:hover,.yaml-completion-item.active{background:#173058}.yaml-completion-value{font:12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;color:#bcd2ff}.yaml-completion-description{color:var(--muted);font-size:11px}.yaml-completion-hint{color:#7f95b5}.section-editor.completion-open{border-color:#587bc1;box-shadow:0 0 0 2px #385b9b33}@media(max-width:760px){.yaml-completion-item{grid-template-columns:1fr;gap:3px}}
.reference-status{margin:10px 0;padding:9px 11px;border:1px solid #29405f;border-radius:8px;background:#0b1727;color:var(--muted);font-size:12px}.reference-status[hidden]{display:none}.reference-status[data-state="checking"],.reference-status[data-state="paused"]{border-color:#475873;color:#aebbd0}.reference-status[data-state="valid"]{border-color:#256d5a;background:#0d2a24;color:#8ce5c7}.reference-status[data-state="warning"],.reference-status[data-state="invalid"]{border-color:#8b6329;background:#302311;color:#ffd08a}.diagnostic-list{display:grid;gap:8px}.diagnostic-item{display:flex;align-items:center;justify-content:space-between;gap:12px}.diagnostic-message{min-width:0}.diagnostic-location{display:block;margin-top:2px;color:#d9ad6d;font:11px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace}.diagnostic-action{flex:0 0 auto;border:1px solid #9a7134;border-radius:7px;background:#3c2a12;color:#ffe0a7;padding:6px 9px;cursor:pointer;font-weight:650}.diagnostic-action:hover{background:#503719}@media(max-width:760px){.diagnostic-item{align-items:flex-start;flex-direction:column}}
.editor-mode-switch{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:14px 0;padding:6px;background:#091523;border:1px solid #243750;border-radius:12px}.editor-mode-actions{display:flex;gap:6px}.editor-mode-button{border:0;background:transparent;color:#91a2bb;border-radius:8px;padding:9px 13px;cursor:pointer;font-weight:650}.editor-mode-button.active{color:#f4f7ff;background:linear-gradient(135deg,#294b91,#3e61ad)}.visual-builder[hidden],.yaml-builder[hidden],.topology-view[hidden]{display:none}.visual-builder{display:grid;gap:14px}.visual-toolbar{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px;padding:14px;border:1px solid #243750;border-radius:10px;background:#091523}.visual-toolbar label{display:grid;gap:6px;color:var(--muted);font-size:12px}.visual-toolbar .visual-add{align-self:end}.visual-pipelines{display:grid;gap:12px}.visual-pipeline{padding:15px;border:1px solid #29405f;border-radius:10px;background:#07111e}.visual-pipeline-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:12px}.visual-flow{display:grid;grid-template-columns:minmax(0,1fr) auto minmax(0,1fr) auto minmax(0,1fr);gap:10px;align-items:stretch}.visual-node{padding:12px;border:1px solid #2b4568;border-radius:9px;background:#0d1b2d}.visual-node-title{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:.08em;margin-bottom:7px}.visual-node-items{display:flex;gap:6px;flex-wrap:wrap}.visual-arrow{display:flex;align-items:center;color:#6f91c5;font-size:22px}.visual-empty{padding:18px;border:1px dashed #365071;border-radius:10px;color:var(--muted);text-align:center}.visual-warning{color:#ffd08a;font-size:12px}@media(max-width:1050px){.visual-toolbar{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:760px){.editor-mode-switch{align-items:flex-start;flex-direction:column}.visual-toolbar{grid-template-columns:1fr}.visual-flow{grid-template-columns:1fr}.visual-arrow{justify-content:center;transform:rotate(90deg)}}
.topology-view{display:grid;gap:14px}.topology-toolbar{display:flex;align-items:end;justify-content:space-between;gap:12px;padding:12px 14px;border:1px solid #243750;border-radius:10px;background:#091523}.topology-toolbar label{display:grid;gap:6px;min-width:190px;color:var(--muted);font-size:12px}.topology-legend{display:flex;flex-wrap:wrap;gap:8px}.topology-legend-item{display:flex;align-items:center;gap:6px;color:var(--muted);font-size:12px}.topology-legend-dot{width:9px;height:9px;border-radius:50%}.topology-legend-dot.receiver{background:#53a8ff}.topology-legend-dot.processor{background:#bd8cff}.topology-legend-dot.exporter{background:#45d6a6}.topology-legend-dot.connector{background:#ffb95c}.topology-canvas{display:grid;gap:13px}.topology-pipeline{padding:15px;border:1px solid #29405f;border-radius:11px;background:#07111e}.topology-pipeline-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:12px}.topology-signal{padding:4px 8px;border-radius:999px;background:#173058;color:#bcd2ff;font-size:11px;text-transform:uppercase;letter-spacing:.06em}.topology-flow{display:flex;align-items:center;gap:8px;overflow-x:auto;padding-bottom:4px}.topology-stage{display:grid;align-self:stretch;gap:7px}.topology-node{min-width:150px;padding:11px;border:1px solid #365071;border-top-width:3px;border-radius:9px;background:#0d1b2d}.topology-node.receiver{border-top-color:#53a8ff}.topology-node.processor{border-top-color:#bd8cff}.topology-node.exporter{border-top-color:#45d6a6}.topology-node.connector{border-top-color:#ffb95c}.topology-node-kind{margin-bottom:5px;color:var(--muted);font-size:10px;text-transform:uppercase;letter-spacing:.08em}.topology-node-name{font:12px/1.4 ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}.topology-edge{display:flex;align-items:center;color:#6f91c5;font-size:20px}.topology-summary{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:8px}.topology-stat{padding:10px;border:1px solid #243750;border-radius:8px;background:#0d1b2d}.topology-stat strong{display:block;font-size:18px}.topology-stat span{color:var(--muted);font-size:11px}@media(max-width:760px){.topology-toolbar{align-items:stretch;flex-direction:column}.topology-toolbar label{min-width:0}.topology-summary{grid-template-columns:repeat(2,minmax(0,1fr))}}
`

const sectionPolicyHTML = `<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>Configuration section policies · FleetAMP</title>
<style>` + controlPlaneCSS + detailCSS + `</style></head><body><div class="shell">` + sideNav + `<main class="main">
<header class="top"><div><div class="crumb">FleetAMP / Settings / Configuration</div>
<div class="pagetitle">Configuration section policies</div>
<div class="subtitle">Control which Collector configuration sections each role may change.</div></div>
<div class="topactions"><a class="btn" href="/settings/users">Users & roles</a></div></header>
<div class="content"><section class="page-description" aria-label="About this page"><div class="page-description-icon" aria-hidden="true">i</div><div><div class="page-description-title">About Configuration Policies</div><p>Define the configuration sections that users may edit and the sections controlled centrally by administrators. These policies constrain configuration generation and validation so group-owned changes cannot modify organization-locked Collector settings.</p></div></section>
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
		editable := principalRole == roleAdmin || ((principalRole == roleOperator || principalRole == roleGroupOwner) && operatorEditable)
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
	if principalRole != roleOperator && principalRole != roleGroupOwner {
		return fmt.Errorf("configuration editing requires Group owner, Operator or Admin access")
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
		validationContent, resolveErr := configurationContentForValidation(r.Context(), request.GroupID, content)
		if resolveErr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, composeSectionsResponse{Validation: configs.ValidationResult{Error: resolveErr.Error()}})
			return
		}
		validation := validator.Validate(r.Context(), validationContent)
		pipeline, pipelineErr := configs.ParsePipelineModel(content)
		components, componentErr := configs.ParseComponentCatalog(content)
		if validation.Valid && pipelineErr != nil {
			validation.Valid = false
			validation.Error = pipelineErr.Error()
		}
		if validation.Valid && componentErr != nil {
			validation.Valid = false
			validation.Error = componentErr.Error()
		}
		status := http.StatusOK
		if !validation.Valid {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, composeSectionsResponse{Content: content, Validation: validation, Pipeline: pipeline, Components: components})
	})

	mux.HandleFunc("POST /api/v1/configurations/sections/references", func(w http.ResponseWriter, r *http.Request) {
		var request composeSectionsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, referenceValidationResponse{Error: "invalid JSON body"})
			return
		}
		content, err := configs.ComposeConfigurationSections(request.Baseline, request.Sections)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, referenceValidationResponse{Error: err.Error(), Diagnostics: referenceDiagnostics(request.Sections, err)})
			return
		}
		pipeline, err := configs.ParsePipelineModel(content)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, referenceValidationResponse{Error: err.Error(), Diagnostics: referenceDiagnostics(request.Sections, err)})
			return
		}
		writeJSON(w, http.StatusOK, referenceValidationResponse{Valid: true, Warnings: pipeline.Warnings})
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
		view := sectionPolicyPageData{Page: "settings-sections", Message: r.URL.Query().Get("message")}
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
