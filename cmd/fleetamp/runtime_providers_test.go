package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestOTelComponentsPageShowsCapabilityMatrix(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/otel-components", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /otel-components status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"OpenTelemetry Collector", "Kubernetes OTel Collector", "OpenTelemetry Operator", "Agent", "Gateway", "DaemonSet", "supported", "planned", "built-in"} {
		if !strings.Contains(body, text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if strings.Contains(body, "Grafana Alloy") {
		t.Error("external add-ons must not appear in the built-in runtime registry")
	}
	if !strings.Contains(body, `class="navitem active" href="/otel-components"`) {
		t.Error("OTel components navigation is not active")
	}
}

func TestOTelComponentsPageRejectsMutation(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/otel-components", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /otel-components status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}

func TestRuntimeProvidersRedirectsToOTelComponents(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/runtime-providers", nil))
	if response.Code != http.StatusPermanentRedirect || response.Header().Get("Location") != "/otel-components" {
		t.Fatalf("legacy route status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
}
