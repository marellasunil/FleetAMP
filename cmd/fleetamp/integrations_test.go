package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/integrations"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type integrationTestSecretResolver struct{ value string }

func (r integrationTestSecretResolver) Resolve(context.Context, string) (string, error) {
	return r.value, nil
}

func integrationTestDB(t *testing.T) *sqlitestore.Database {
	t.Helper()
	db, e := sqlitestore.Open(context.Background(), filepath.Join(t.TempDir(), "integrations.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
func TestIntegrationsPageRegistersRepositoryScope(t *testing.T) {
	db := integrationTestDB(t)
	g, e := groups.New("Payments", "", nil)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Groups().Create(t.Context(), g); e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog(), db.IntegrationConnections(), db.IntegrationValidations(), &integrations.ConnectionValidator{}, 24*time.Hour, db.Groups(), nil)
	form := url.Values{"name": {"production"}, "provider": {"github"}, "organization": {"acme"}, "repository": {"telemetry"}, "branch": {"main"}, "allowed_root": {"fleetamp/groups"}, "mode": {"fleetamp-pull-request"}, "credential_ref": {"secret://github/prod"}, "group_ids": {g.ID}, "enabled": {"true"}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/integrations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	rows, e := db.IntegrationConnections().List(t.Context(), 10)
	if e != nil || len(rows) != 1 || rows[0].CredentialRef != "secret://github/prod" {
		t.Fatalf("rows=%#v err=%v", rows, e)
	}
}
func TestIntegrationsPageShowsSafetyBoundary(t *testing.T) {
	db := integrationTestDB(t)
	mux := http.NewServeMux()
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog(), db.IntegrationConnections(), db.IntegrationValidations(), &integrations.ConnectionValidator{}, 24*time.Hour, db.Groups(), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings/integrations", nil))
	for _, want := range []string{"GitHub", "GitLab", "Azure DevOps", "Register repository connection", "Governed provider access", "secret reference only"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestIntegrationsPageTestsConnectionAndStoresEvidence(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/telemetry":
			_, _ = w.Write([]byte(`{"full_name":"acme/telemetry","permissions":{"pull":true,"push":true}}`))
		case "/repos/acme/telemetry/branches/main":
			_, _ = w.Write([]byte(`{"name":"main"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	db := integrationTestDB(t)
	connection, _ := integrations.NewConnection(integrations.Connection{Name: "production", Provider: integrations.GitHub, BaseURL: provider.URL, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: integrations.ModePullRequest, CredentialRef: "secret://github/prod", GroupIDs: []string{"payments"}, Enabled: true}, "admin")
	if err := db.IntegrationConnections().Create(t.Context(), connection); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	validator := &integrations.ConnectionValidator{Secrets: integrationTestSecretResolver{"never-render-this-token"}, Client: provider.Client()}
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog(), db.IntegrationConnections(), db.IntegrationValidations(), validator, time.Hour, db.Groups(), nil)
	form := url.Values{"action": {"test_connection"}, "connection_id": {connection.ID}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/integrations", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	record, err := db.IntegrationValidations().Latest(t.Context(), connection.ID)
	if err != nil || record.Status != "passed" || record.Evidence["repository"] != "acme/telemetry" {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/settings/integrations", nil))
	if strings.Contains(page.Body.String(), "never-render-this-token") || !strings.Contains(page.Body.String(), "Provider repository and branch validation passed") {
		t.Fatalf("unexpected page: %s", page.Body.String())
	}
}
