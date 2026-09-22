package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

func TestGroupDetailShowsSelectedConfigurationPipelineAndTargets(t *testing.T) {
	content := `
receivers:
  hostmetrics:
processors:
  batch:
exporters:
  otlp/central:
    endpoint: central.example.invalid:4317
    headers:
      authorization: hidden-secret
service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [otlp/central]
`
	configuration := configs.NewConfiguration("agent.yaml", "2.1.0", content, "text/yaml")
	pipeline, err := configs.ParsePipelineModel(content)
	if err != nil {
		t.Fatal(err)
	}
	group := &groups.Group{ID: "group-1", Name: "payments · prod · eu", Enabled: true}
	agent := &agents.ManagedAgent{InstanceUID: "agent-1", Name: "collector-1", Connected: true, Healthy: true}
	view := groupDetailView{
		Page: "groups", Group: group, Members: []*agents.ManagedAgent{agent},
		Configurations: []*configs.Configuration{configuration}, SelectedConfig: configuration,
		SelectedPipeline: pipeline, Preview: []groupPreviewAgent{{Agent: agent, Reason: "Ready"}}, Eligible: 1,
	}

	var output bytes.Buffer
	if err := groupDetailPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, want := range []string{
		"Review the target, configuration flow and readiness before approval",
		"metrics telemetry", "hostmetrics", "batch", "otlp/central",
		"Target: payments · prod · eu", "1 ready", "collector-1", "Request approval",
		"/configurations/" + configuration.ID,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if strings.Contains(body, "hidden-secret") || strings.Contains(body, "central.example.invalid") {
		t.Fatal("group preview exposed component settings")
	}
}
