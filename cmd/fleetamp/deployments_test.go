package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeploymentsPageDefinesGovernedDeliveryFoundation(t *testing.T) {
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/deployments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /deployments status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, text := range []string{"Configuration deployment", "Component installation", "Component upgrade", "Component removal", "Approve", "exact content hash"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if !strings.Contains(response.Body.String(), `class="navitem active" href="/deployments"`) {
		t.Error("deployments navigation is not active")
	}
}

func TestDeploymentsPageRejectsMutation(t *testing.T) {
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/deployments", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /deployments status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
