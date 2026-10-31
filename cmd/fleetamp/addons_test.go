package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/addons"
)

func TestVendorIntegrationsPageShowsSupportedDestinations(t *testing.T) {
	mux := http.NewServeMux()
	registerAddonRoutes(mux, addons.NewDefaultCatalog())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/addons", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /addons status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"Vendor Integrations", "Splunk integration", "Grafana integration", "Grafana Alloy integration", "Integration boundary"} {
		if !strings.Contains(body, text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	for _, removed := range []string{"Documentation", "Open add-ons repository", "Installation unavailable"} {
		if strings.Contains(body, removed) {
			t.Errorf("instructional action %q is still visible", removed)
		}
	}
	if !strings.Contains(body, `class="navitem active" href="/addons"`) {
		t.Error("add-ons navigation is not active")
	}
}

func TestAddonsPageRejectsMutation(t *testing.T) {
	mux := http.NewServeMux()
	registerAddonRoutes(mux, addons.NewDefaultCatalog())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/addons", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /addons status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
