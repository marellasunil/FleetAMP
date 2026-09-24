package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestSectionViewsApplyRoleAndDefaultPolicy(t *testing.T) {
	content := "receivers:\n  otlp: {}\nexporters:\n  debug: {}\nservice:\n  pipelines: {}\n"
	policies := configs.DefaultSectionPolicies()

	operatorViews, err := buildConfigurationSectionViews(content, policies, roleOperator)
	if err != nil {
		t.Fatal(err)
	}
	adminViews, err := buildConfigurationSectionViews(content, policies, roleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	operatorAccess := make(map[string]bool)
	for _, view := range operatorViews {
		operatorAccess[view.Key] = view.Editable
	}
	if !operatorAccess[configs.SectionReceivers] {
		t.Fatal("receivers should be editable by Operators by default")
	}
	if operatorAccess[configs.SectionExporters] || operatorAccess[configs.SectionServicePipelines] {
		t.Fatal("exporters and service pipelines must be read-only for Operators by default")
	}
	for _, view := range adminViews {
		if !view.Editable {
			t.Fatalf("Admin cannot edit %s", view.Key)
		}
	}
}

func TestSectionPolicyRejectsProtectedOperatorChange(t *testing.T) {
	before := "receivers:\n  otlp: {}\nexporters:\n  debug: {}\nservice:\n  pipelines: {}\n"
	after := "receivers:\n  otlp: {}\nexporters:\n  otlphttp:\n    endpoint: https://example.invalid\nservice:\n  pipelines: {}\n"
	err := enforceSectionPolicies(before, after, configs.DefaultSectionPolicies(), roleOperator)
	if err == nil || !strings.Contains(err.Error(), "Exporters") {
		t.Fatalf("error=%v, want protected Exporters error", err)
	}
	if err := enforceSectionPolicies(before, after, configs.DefaultSectionPolicies(), roleAdmin); err != nil {
		t.Fatalf("Admin edit rejected: %v", err)
	}
}

func TestComposeSectionsEndpointValidatesCompleteDocument(t *testing.T) {
	mux := http.NewServeMux()
	registerSectionEditorRoutes(mux, nil, configs.NewValidator(""), nil)
	body := `{"baseline":"custom_key: preserved\n","sections":{"receivers":"otlp: {}\n","service_pipelines":"traces:\n  receivers: [otlp]\n  exporters: [debug]\n","exporters":"debug: {}\n"}}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/configurations/sections/compose", strings.NewReader(body))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, expected := range []string{"custom_key", "receivers", "pipelines", "exporters", `"valid":true`} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("response is missing %q: %s", expected, response.Body.String())
		}
	}
}

func TestConfigurationSectionSettingsAndRawCreationRequireAdmin(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/settings/configuration-sections", nil)
	if got := requiredPermission(request); got != permissionAdmin {
		t.Fatalf("settings permission=%q, want admin", got)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/configurations", nil)
	if got := requiredPermission(request); got != permissionAdmin {
		t.Fatalf("raw creation permission=%q, want admin", got)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/configurations/sections/compose", nil)
	if got := requiredPermission(request); got != permissionEdit {
		t.Fatalf("section compose permission=%q, want edit", got)
	}
}

func TestAgentDetailRendersAllConfigurationSectionTabs(t *testing.T) {
	views, err := buildConfigurationSectionViews(
		"receivers:\n  otlp: {}\nservice:\n  pipelines: {}\n",
		configs.DefaultSectionPolicies(),
		roleAdmin,
	)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = agentDetailPage.Execute(&output, agentDetailView{
		Page: "fleet",
		Agent: &agents.ManagedAgent{
			InstanceUID: "agent-one",
			Name:        "collector-one",
		},
		EditorSections:       views,
		EditorBaseline:       "receivers:\n  otlp: {}\nservice:\n  pipelines: {}\n",
		CanEditConfiguration: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, definition := range configs.ConfigurationSectionDefinitions {
		if !strings.Contains(rendered, `data-section-tab="`+definition.Key+`"`) {
			t.Fatalf("rendered editor is missing %s tab", definition.Title)
		}
	}
	if !strings.Contains(rendered, "Complete YAML") ||
		!strings.Contains(rendered, "Validate and save version") {
		t.Fatal("rendered editor is missing preview or save controls")
	}
}
