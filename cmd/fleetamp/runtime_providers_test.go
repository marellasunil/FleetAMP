package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
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

func TestOTelComponentsInstalledInventoryUsesManagedAgents(t *testing.T) {
	store := memory.NewAgentStore()
	agent := &agents.ManagedAgent{
		InstanceUID: "collector-1",
		Type:        agents.AgentTypeOTelCollector,
		Name:        "payments-gateway",
		Version:     "0.149.0",
		Connected:   true,
		Healthy:     true,
		Status:      agents.LifecycleConnected,
		LastSeen:    time.Date(2026, time.October, 6, 7, 0, 0, 0, time.UTC),
		Deployment: agents.DeploymentContext{
			Runtime:   agents.RuntimeKubernetes,
			Cluster:   "demo-cluster",
			Namespace: "observability",
		},
		Attributes: map[string]string{
			"fleetamp.component.role": "Gateway",
			"host.arch":               "amd64",
			"k8s.deployment.name":     "otel-gateway",
		},
	}
	if err := store.Upsert(context.Background(), agent); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry(), store)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/otel-components?tab=installed", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET installed inventory status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, text := range []string{"payments-gateway", "OTel Collector", "Gateway", "0.149.0", "kubernetes / amd64", "Deployment", "demo-cluster / observability", "OpAMP", "Healthy"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("installed inventory does not contain %q", text)
		}
	}
}

func TestInstalledInventorySearchDoesNotChangeSummary(t *testing.T) {
	items := []*agents.ManagedAgent{
		{InstanceUID: "uid-one", Name: "alpha", Connected: true, Healthy: true},
		{InstanceUID: "uid-two", Name: "beta", Connected: false, Healthy: false},
	}
	filtered, total, healthy := installedComponentInventory(items, "uid-one")
	if len(filtered) != 1 || total != 2 || healthy != 1 {
		t.Fatalf("inventory summary filtered=%d total=%d healthy=%d", len(filtered), total, healthy)
	}
}

func TestOTelComponentsCompatibilityUsesReportedCapabilities(t *testing.T) {
	store := memory.NewAgentStore()
	agent := &agents.ManagedAgent{
		InstanceUID: "compatible-collector",
		Type:        agents.AgentTypeOTelCollector,
		Name:        "checkout-gateway",
		Version:     "0.149.0",
		Connected:   true,
		Healthy:     true,
		Status:      agents.LifecycleConnected,
		Deployment: agents.DeploymentContext{
			Runtime:   agents.RuntimeKubernetes,
			Cluster:   "production",
			Namespace: "observability",
		},
		Capabilities: []string{"accepts_remote_config", "reports_effective_config", "reports_health"},
	}
	if err := store.Upsert(context.Background(), agent); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry(), store)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/otel-components?tab=compatibility", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET compatibility status = %d, want %d", response.Code, http.StatusOK)
	}
	for _, text := range []string{"Management compatibility", "checkout-gateway", "Compatible", "Remote configuration", "Effective configuration", "Kubernetes context"} {
		if !strings.Contains(response.Body.String(), text) {
			t.Errorf("compatibility page does not contain %q", text)
		}
	}
}

func TestCompatibilityAssessmentSeparatesAttentionAndUnsupported(t *testing.T) {
	items := []*agents.ManagedAgent{
		{InstanceUID: "ready", Type: agents.AgentTypeOTelCollector, Name: "ready", Version: "0.149.0", Deployment: agents.DeploymentContext{Runtime: agents.RuntimeVM}, Capabilities: []string{"accepts_remote_config", "reports_effective_config", "reports_health"}},
		{InstanceUID: "missing", Type: agents.AgentTypeOTelCollector, Name: "missing"},
		{InstanceUID: "external", Type: agents.AgentTypeGrafanaAlloy, Name: "external"},
	}
	assessments, total, compatible, attention, unsupported, findings := assessComponentCompatibility(items)
	if len(assessments) != 3 || total != 3 || compatible != 1 || attention != 1 || unsupported != 1 || findings == 0 {
		t.Fatalf("compatibility len=%d total=%d compatible=%d attention=%d unsupported=%d findings=%d", len(assessments), total, compatible, attention, unsupported, findings)
	}
	if assessments[0].Status != "Unsupported" {
		t.Fatalf("first assessment status = %q, want Unsupported", assessments[0].Status)
	}
}

func TestOTelComponentsInstallationGuidesAreReadOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerRuntimeProviderRoutes(mux, runtimes.NewDefaultRegistry())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/otel-components?tab=guides", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET installation guides status = %d, want %d", response.Code, http.StatusOK)
	}
	body := response.Body.String()
	for _, text := range []string{"Choose an installation model", "Linux system service", "Container runtime", "Kubernetes workload", "OpenTelemetry Operator", "GitOps-managed Kubernetes", "Prerequisites", "Recommended sequence", "Verify before governance", "Read-only guidance"} {
		if !strings.Contains(body, text) {
			t.Errorf("installation guides page does not contain %q", text)
		}
	}
	for _, forbidden := range []string{"Install now", "Run installation", "Connect repository"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("installation guides page contains mutation control %q", forbidden)
		}
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
