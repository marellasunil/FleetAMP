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
)

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
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog(), db.IntegrationConnections(), db.Groups(), nil)
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
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog(), db.IntegrationConnections(), db.Groups(), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings/integrations", nil))
	for _, want := range []string{"GitHub", "GitLab", "Azure DevOps", "Register repository connection", "No provider API calls", "secret reference only"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
