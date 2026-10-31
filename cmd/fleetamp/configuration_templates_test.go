package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

const reusableTemplateYAML = `receivers:
  otlp:
    protocols:
      grpc: {}
exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`

func TestConfigurationTemplateFromSavedVersionCreatesNewGroupVersion(t *testing.T) {
	db, err := sqlitestore.Open(t.Context(), t.TempDir()+"/fleetamp.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	group, err := groups.New("Payments", "", map[string]string{"team": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Groups().Create(t.Context(), group); err != nil {
		t.Fatal(err)
	}
	source := configs.NewGroupConfiguration(group.ID, "payments-source", "1.0.0", reusableTemplateYAML, "text/yaml")
	if err := db.Configurations().Put(t.Context(), source); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerConfigurationTemplateRoutes(mux, db.ConfigurationTemplates(), db.Configurations(), db.Groups(), configs.NewValidator(""), nil)

	create := url.Values{
		"action":           {"create"},
		"name":             {"Payments standard"},
		"version":          {"1.0.0"},
		"description":      {"Reusable payment telemetry"},
		"source":           {"existing"},
		"configuration_id": {source.ID},
		"visibility":       {string(configs.TemplateSelected)},
		"group_ids":        {group.ID},
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/blueprints/templates", strings.NewReader(create.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	templates, err := db.ConfigurationTemplates().List(t.Context())
	if err != nil || len(templates) != 1 {
		t.Fatalf("unexpected templates: %#v err=%v", templates, err)
	}
	if templates[0].SourceConfigurationID != source.ID || templates[0].Content != source.Content {
		t.Fatalf("source provenance was not preserved: %#v", templates[0])
	}

	instantiate := url.Values{
		"action":                {"instantiate"},
		"template_id":           {templates[0].ID},
		"group_id":              {group.ID},
		"configuration_name":    {"payments-from-template"},
		"configuration_version": {"2.0.0"},
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/blueprints/templates", strings.NewReader(instantiate.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || !strings.HasPrefix(response.Header().Get("Location"), "/groups/"+group.ID) {
		t.Fatalf("instantiate status=%d location=%q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	configurations, err := db.Configurations().List(t.Context())
	if err != nil || len(configurations) != 2 {
		t.Fatalf("unexpected configurations: %#v err=%v", configurations, err)
	}
	var created *configs.Configuration
	for _, configuration := range configurations {
		if configuration.Name == "payments-from-template" {
			created = configuration
		}
	}
	if created == nil || created.GroupID != group.ID || created.Content != reusableTemplateYAML {
		t.Fatalf("template did not create expected group version: %#v", created)
	}
}

func TestConfigurationTemplatesPageSupportsManualCreation(t *testing.T) {
	db, err := sqlitestore.Open(t.Context(), t.TempDir()+"/fleetamp.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	group, _ := groups.New("Payments", "", nil)
	if err := db.Groups().Create(t.Context(), group); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerConfigurationTemplateRoutes(mux, db.ConfigurationTemplates(), db.Configurations(), db.Groups(), configs.NewValidator(""), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/blueprints/templates", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status=%d body=%s", response.Code, response.Body.String())
	}
	for _, expected := range []string{"Configuration Templates", "Create manually", "Existing saved configuration", "Selected groups", "Organization-wide"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("page is missing %q", expected)
		}
	}
}
