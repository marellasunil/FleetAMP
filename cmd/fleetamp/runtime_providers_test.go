package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestRuntimeProvidersPageShowsCapabilityMatrix(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/runtime-providers", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /runtime-providers status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"OpenTelemetry Collector", "Kubernetes OTel Collector", "Grafana Alloy", "supported", "planned", "add-on", "does not bundle or install Alloy", "GitHub, GitLab, and Azure DevOps"} {
		if !strings.Contains(body, text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if !strings.Contains(body, `class="navitem active" href="/runtime-providers"`) {
		t.Error("runtime provider navigation is not active")
	}
}

func TestRuntimeProvidersPageRejectsMutation(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/runtime-providers", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /runtime-providers status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
