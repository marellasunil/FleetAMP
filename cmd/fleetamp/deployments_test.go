package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func TestDeploymentsPageDefinesGovernedDeliveryFoundation(t *testing.T) {
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, memory.NewComponentLifecycleRequestStore(), memory.NewComponentLifecycleValidationStore(), memory.NewComponentLifecycleApprovalStore(), nil, memory.NewAgentStore(), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/deployments", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /deployments status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, text := range []string{"Create immutable component proposal", "Install", "Upgrade", "Restart", "Remove", "Approve", "exact specification hash"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("page does not contain %q", text)
		}
	}
	if !strings.Contains(response.Body.String(), `class="navitem active" href="/deployments"`) {
		t.Error("deployments navigation is not active")
	}
	for _, text := range []string{
		`.cardbody>form>.form-grid{grid-template-columns:minmax(0,760px)`,
		`Choose whether to install, upgrade, restart or remove the component.`,
		`Select the type of OpenTelemetry component this proposal will manage.`,
		`Required for install and upgrade proposals.`,
	} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("vertical proposal form does not contain %q", text)
		}
	}
}

func TestDeploymentsPageCreatesImmutableLifecycleProposal(t *testing.T) {
	store := memory.NewComponentLifecycleRequestStore()
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, store, memory.NewComponentLifecycleValidationStore(), memory.NewComponentLifecycleApprovalStore(), nil, memory.NewAgentStore(), nil)
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
	registerDeploymentRoutes(mux, memory.NewComponentLifecycleRequestStore(), memory.NewComponentLifecycleValidationStore(), memory.NewComponentLifecycleApprovalStore(), nil, memory.NewAgentStore(), nil)
	form := url.Values{"operation": {"upgrade"}, "component_type": {"otel-collector"}, "group_id": {"payments"}, "deployment_method": {"gitops"}, "current_version": {"0.149.0"}, "desired_version": {"0.149.0"}, "reason": {"no change"}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "different current and desired versions") {
		t.Fatalf("invalid proposal status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDeploymentsPageCreatesImmutableValidationSnapshot(t *testing.T) {
	ctx := context.Background()
	database, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "deployments.db"))
	if err != nil { t.Fatal(err) }
	defer database.Close()
	group, err := groups.New("Payments", "", map[string]string{"team": "payments"})
	if err != nil { t.Fatal(err) }
	if err := database.Groups().Create(ctx, group); err != nil { t.Fatal(err) }
	requests := memory.NewComponentLifecycleRequestStore()
	request, err := lifecycle.NewRequest(lifecycle.Spec{Operation: lifecycle.Upgrade, ComponentType: runtimes.OTelCollectorKubernetes,
		GroupID: group.ID, LabelSelector: "environment=production", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security update"}, "test-user")
	if err != nil { t.Fatal(err) }
	if err := requests.Create(ctx, request); err != nil { t.Fatal(err) }
	agentsStore := memory.NewAgentStore()
	if err := agentsStore.Upsert(ctx, &agents.ManagedAgent{InstanceUID: "collector-a", Type: agents.AgentTypeOTelCollector, Version: "0.148.0",
		Connected: true, Healthy: true, Deployment: agents.DeploymentContext{Runtime: agents.RuntimeKubernetes}, GroupFields: map[string]string{"team": "payments"},
		Labels: map[string]string{"environment": "production"}, Capabilities: []string{"reports_health"}}); err != nil { t.Fatal(err) }
	validations := memory.NewComponentLifecycleValidationStore()
	mux := http.NewServeMux()
	registerDeploymentRoutes(mux, requests, validations, memory.NewComponentLifecycleApprovalStore(), database.Groups(), agentsStore, nil)
	form := url.Values{"action": {"validate"}, "request_id": {request.ID}}
	response := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/deployments", strings.NewReader(form.Encode()))
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, httpRequest)
	if response.Code != http.StatusSeeOther { t.Fatalf("validation status=%d body=%s", response.Code, response.Body.String()) }
	items, err := validations.ListByRequest(ctx, request.ID)
	if err != nil || len(items) != 1 || items[0].Status != lifecycle.ValidationCompatible { t.Fatalf("validations=%#v err=%v", items, err) }
}
