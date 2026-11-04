package configs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePipelineModel(t *testing.T) {
	content := `
receivers:
  otlp/internal:
    protocols:
      grpc:
  hostmetrics:
processors:
  memory_limiter:
  batch/production:
exporters:
  otlp/grafana:
    endpoint: example.invalid:4317
    headers:
      authorization: secret-value
  debug/unused:
extensions:
  health_check:
  pprof/unused:
service:
  extensions: [health_check]
  pipelines:
    traces/application:
      receivers: [otlp/internal]
      processors: [memory_limiter, batch/production]
      exporters: [otlp/grafana]
    metrics:
      receivers: [hostmetrics]
      processors: [batch/production]
      exporters: [otlp/grafana]
`

	model, err := ParsePipelineModel(content)
	if err != nil {
		t.Fatalf("ParsePipelineModel() error = %v", err)
	}
	if len(model.Pipelines) != 2 {
		t.Fatalf("got %d pipelines, want 2", len(model.Pipelines))
	}
	if model.Pipelines[0].Name != "metrics" || model.Pipelines[1].Name != "traces/application" {
		t.Fatalf("pipelines are not deterministic: %#v", model.Pipelines)
	}
	if got := strings.Join(model.Pipelines[1].Processors, ","); got != "memory_limiter,batch/production" {
		t.Fatalf("processor order = %q", got)
	}
	if got := strings.Join(model.Extensions, ","); got != "health_check" {
		t.Fatalf("extensions = %q", got)
	}
	warnings := strings.Join(model.Warnings, "\n")
	for _, want := range []string{
		`exporter "debug/unused" is defined but unused`,
		`extension "pprof/unused" is defined but unused`,
	} {
		if !strings.Contains(warnings, want) {
			t.Errorf("warnings %q do not contain %q", warnings, want)
		}
	}
	data, err := json.Marshal(model)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") {
		t.Fatal("pipeline model exposed component settings")
	}
}

func TestParseComponentCatalogReturnsNamesWithoutSettings(t *testing.T) {
	content := `
receivers:
  zipkin:
  otlp/internal:
    protocols:
      grpc:
processors:
  batch:
exporters:
  otlphttp/production:
    endpoint: https://example.invalid
    headers:
      authorization: secret-value
extensions:
  health_check:
connectors:
  forward:
`

	catalog, err := ParseComponentCatalog(content)
	if err != nil {
		t.Fatalf("ParseComponentCatalog() error = %v", err)
	}
	if got := strings.Join(catalog.Receivers, ","); got != "forward,otlp/internal,zipkin" {
		t.Fatalf("receivers = %q", got)
	}
	if got := strings.Join(catalog.Processors, ","); got != "batch" {
		t.Fatalf("processors = %q", got)
	}
	if got := strings.Join(catalog.Exporters, ","); got != "forward,otlphttp/production" {
		t.Fatalf("exporters = %q", got)
	}
	if got := strings.Join(catalog.Extensions, ","); got != "health_check" {
		t.Fatalf("extensions = %q", got)
	}
	if got := strings.Join(catalog.Connectors, ","); got != "forward" {
		t.Fatalf("connectors = %q", got)
	}
	data, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") || strings.Contains(string(data), "example.invalid") {
		t.Fatal("component catalog exposed component settings")
	}
}

func TestParsePipelineModelAcceptsConnectorEndpoints(t *testing.T) {
	content := `
receivers:
  otlp:
exporters:
  debug:
connectors:
  forward:
service:
  pipelines:
    traces/in:
      receivers: [otlp]
      exporters: [forward]
    traces/out:
      receivers: [forward]
      exporters: [debug]
`
	model, err := ParsePipelineModel(content)
	if err != nil {
		t.Fatalf("ParsePipelineModel() error = %v", err)
	}
	if len(model.Pipelines) != 2 {
		t.Fatalf("got %d pipelines, want 2", len(model.Pipelines))
	}
	if warnings := strings.Join(model.Warnings, "\n"); strings.Contains(warnings, "connector") {
		t.Fatalf("used connector reported as unused: %s", warnings)
	}
}

func TestParsePipelineModelRejectsBrokenRelationships(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{
			name: "missing receiver",
			content: `service:
  pipelines:
    metrics:
      receivers: [missing]
      exporters: [debug]
exporters:
  debug:
`,
			want: `pipeline "metrics" references undefined receiver "missing"`,
		},
		{
			name: "missing exporter",
			content: `receivers:
  otlp:
service:
  pipelines:
    traces:
      receivers: [otlp]
`,
			want: `pipeline "traces" has no exporters`,
		},
		{
			name: "missing extension",
			content: `service:
  extensions: [health_check]
  pipelines: {}
`,
			want: `service references undefined extension "health_check"`,
		},
		{
			name: "unsupported signal",
			content: `receivers:
  otlp:
exporters:
  debug:
service:
  pipelines:
    events:
      receivers: [otlp]
      exporters: [debug]
`,
			want: `pipeline "events" uses unsupported signal "events"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePipelineModel(tt.content)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParsePipelineModelAllowsNoPipelines(t *testing.T) {
	model, err := ParsePipelineModel("service:\n  telemetry:\n    logs:\n      level: info\n")
	if err != nil {
		t.Fatalf("ParsePipelineModel() error = %v", err)
	}
	if len(model.Pipelines) != 0 {
		t.Fatalf("got %d pipelines, want none", len(model.Pipelines))
	}
}
