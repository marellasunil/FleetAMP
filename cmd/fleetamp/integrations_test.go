package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/integrations"
)

func TestIntegrationsPageShowsProviderFoundation(t *testing.T) {
	mux := http.NewServeMux()
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings/integrations", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /settings/integrations status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"GitHub", "GitLab", "Azure DevOps", "No repository creation", "No pipeline creation", "Approval required", "Connect · Next milestone"} {
		if !strings.Contains(body, text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if !strings.Contains(body, `class="navitem active admin-settings-link" href="/settings/integrations"`) {
		t.Error("integrations navigation is not active")
	}
}

func TestIntegrationsPageRejectsMutation(t *testing.T) {
	mux := http.NewServeMux()
	registerIntegrationRoutes(mux, integrations.NewDefaultGitCatalog())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/settings/integrations", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /settings/integrations status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
