package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestDeploymentsPageDefinesGovernedDeliveryFoundation(t *testing.T) {
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, memory.NewComponentLifecycleRequestStore(), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/deployments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /deployments status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, text := range []string{"Create immutable component proposal", "Component installation", "Component upgrade", "Component restart", "Component removal", "Approve", "exact specification hash"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if !strings.Contains(response.Body.String(), `class="navitem active" href="/deployments"`) {
		t.Error("deployments navigation is not active")
	}
}

func TestDeploymentsPageCreatesImmutableLifecycleProposal(t *testing.T) {
	store := memory.NewComponentLifecycleRequestStore()
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, store, nil)
	form := url.Values{
		"operation": {"upgrade"}, "component_type": {"otel-collector-kubernetes"}, "group_id": {"payments"},
		"label_selector": {"environment=production"}, "deployment_method": {"gitops"},
		"current_version": {"0.148.0"}, "desired_version": {"0.149.0"}, "reason": {"approved security update"},
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST /deployments status = %d, want %d: %s", response.Code, http.StatusSeeOther, response.Body.String())
	}
	items, err := store.List(context.Background(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("stored proposals=%d err=%v", len(items), err)
	}
	if items[0].Spec.Operation != "upgrade" || items[0].SpecHash == "" || items[0].Status != "proposed" {
		t.Fatalf("unexpected stored proposal: %#v", items[0])
	}
}

func TestDeploymentsPageRejectsInvalidLifecycleProposal(t *testing.T) {
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, memory.NewComponentLifecycleRequestStore(), nil)
	form := url.Values{"operation": {"upgrade"}, "component_type": {"otel-collector"}, "group_id": {"payments"}, "deployment_method": {"gitops"}, "current_version": {"0.149.0"}, "desired_version": {"0.149.0"}, "reason": {"no change"}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "different current and desired versions") {
		t.Fatalf("invalid proposal status=%d body=%s", response.Code, response.Body.String())
	}
}
