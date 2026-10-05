package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/addons"
)

func TestAddonsPageIsReadOnlyCatalog(t *testing.T) {
	mux := http.NewServeMux()
	registerAddonRoutes(mux, addons.NewDefaultCatalog())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/addons", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /addons status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"Read-only catalog", "Grafana Alloy integration", "External and unbundled", "Installation unavailable", "FleetAMP-Addons"} {
		if !strings.Contains(body, text) {
			t.Errorf("page does not contain %q", text)
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
