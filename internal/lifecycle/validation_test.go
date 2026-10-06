package lifecycle

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestValidateRequestSnapshotsResolvedTargets(t *testing.T) {
	request, err := NewRequest(Spec{Operation: Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments",
		LabelSelector: "environment=production", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security update"}, "operator")
	if err != nil { t.Fatal(err) }
	group := &groups.Group{ID: "payments", Name: "Payments", Enabled: true, Selector: map[string]string{"team": "payments"}}
	inventory := []*agents.ManagedAgent{{InstanceUID: "collector-a", Name: "collector-a", Type: agents.AgentTypeOTelCollector, Version: "0.148.0",
		Connected: true, Healthy: true, Deployment: agents.DeploymentContext{Runtime: agents.RuntimeKubernetes, Cluster: "prod", Namespace: "observability"},
		GroupFields: map[string]string{"team": "payments"}, Labels: map[string]string{"environment": "production"}, Capabilities: []string{"reports_health"}},
		{InstanceUID: "collector-b", Type: agents.AgentTypeOTelCollector, Version: "0.148.0", Deployment: agents.DeploymentContext{Runtime: agents.RuntimeKubernetes},
			GroupFields: map[string]string{"team": "payments"}, Labels: map[string]string{"environment": "staging"}}}
	validation, err := ValidateRequest(request, group, inventory, "admin")
	if err != nil { t.Fatal(err) }
	if validation.Status != ValidationCompatible || len(validation.Targets) != 1 || validation.Targets[0].InstanceUID != "collector-a" || validation.ResultHash == "" {
		t.Fatalf("unexpected validation: %#v", validation)
	}
	inventory[0].Labels["environment"] = "changed"
	if validation.Targets[0].Labels["environment"] != "production" { t.Fatal("validation target snapshot changed with live inventory") }
}

func TestValidateRequestBlocksVersionAndRuntimeMismatch(t *testing.T) {
	request, err := NewRequest(Spec{Operation: Restart, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments",
		DeploymentMethod: "gitops", CurrentVersion: "0.149.0", Reason: "recover stalled component"}, "operator")
	if err != nil { t.Fatal(err) }
	group := &groups.Group{ID: "payments", Name: "Payments", Enabled: true, Selector: map[string]string{"team": "payments"}}
	validation, err := ValidateRequest(request, group, []*agents.ManagedAgent{{InstanceUID: "collector-a", Type: agents.AgentTypeOTelCollector, Version: "0.148.0",
		Deployment: agents.DeploymentContext{Runtime: agents.RuntimeVM}, GroupFields: map[string]string{"team": "payments"}}}, "admin")
	if err != nil { t.Fatal(err) }
	if validation.Status != ValidationBlocked || len(validation.Findings) < 2 { t.Fatalf("unexpected validation: %#v", validation) }
}

func TestParseLabelSelectorRejectsAmbiguity(t *testing.T) {
	if _, err := ParseLabelSelector("environment=prod,environment=stage"); err == nil { t.Fatal("duplicate selector key accepted") }
	if _, err := ParseLabelSelector("environment in (prod)"); err == nil { t.Fatal("unsupported selector syntax accepted") }
}
