package main

import (
	"strings"
	"testing"
)

func TestFleetViewsExplainTheirCapabilities(t *testing.T) {
	for _, expected := range []string{
		"About the Agents view",
		"Monitor OpenTelemetry Collectors registered with FleetAMP through OpAMP.",
		"About the Pipelines view",
		"About AI Insights",
		"About SLOs &amp; Alerts",
		"does not yet generate live findings or make changes to the fleet",
	} {
		if !strings.Contains(fleetHTML, expected) {
			t.Errorf("Fleet page is missing contextual description %q", expected)
		}
	}
}

func TestAgentDetailExplainsOperationalScope(t *testing.T) {
	for _, expected := range []string{
		"About this Collector",
		"reported runtime metadata, connectivity, health and management capabilities",
		"compare desired and effective configuration",
		"never change the Collector's OpAMP identity",
	} {
		if !strings.Contains(agentDetailHTML, expected) {
			t.Errorf("Agent detail page is missing contextual description %q", expected)
		}
	}
}
