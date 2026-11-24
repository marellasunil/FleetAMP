package opamp

import (
	"testing"

	"github.com/marellasunil/OTelPlane/internal/agents"
)

func TestSplitOTelPlaneMetadataAcceptsFlexibleGroupKeys(t *testing.T) {
	groups, labels, unknown := splitOTelPlaneMetadata(map[string]string{
		"otelplane.group.team":         "payments",
		"otelplane.group.cloud.region": "eu-west-1",
		"otelplane.label.release-ring": "canary",
	})
	if groups["team"] != "payments" || groups["cloud.region"] != "eu-west-1" {
		t.Fatalf("unexpected group fields: %#v", groups)
	}
	if labels["release-ring"] != "canary" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown fields: %#v", unknown)
	}
}

func TestMergeAgentPreservesReportedNameOnPartialMessage(t *testing.T) {
	previous := &agents.ManagedAgent{
		InstanceUID: "agent-1",
		Name:        "payments-prod-collector-01",
		Hostname:    "payments-prod-collector-01",
		Version:     "0.149.0",
		Attributes: map[string]string{
			"host.name":         "payments-prod-collector-01",
			"otelplane.agent.id": "payments-prod-collector-01",
		},
	}
	current := &agents.ManagedAgent{
		InstanceUID: "agent-1",
		Name:        "otel-collector-agent-1",
		Connected:   true,
		Healthy:     true,
	}

	mergeAgent(current, previous)

	if current.Name != previous.Name || current.Hostname != previous.Hostname {
		t.Fatalf("partial update replaced reported identity: %#v", current)
	}
	if current.Attributes["otelplane.agent.id"] != "payments-prod-collector-01" {
		t.Fatalf("partial update lost attributes: %#v", current.Attributes)
	}
}
