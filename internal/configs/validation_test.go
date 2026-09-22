// Tests YAML and optional Collector-binary configuration validation.
package configs

import (
	"context"
	"strings"
	"testing"
)

func TestValidatorAcceptsValidYAMLWithoutCollector(t *testing.T) {
	result := NewValidator("").Validate(context.Background(), "service:\n  telemetry:\n    logs:\n      level: info\n")
	if !result.Valid || !result.YAMLValid || !result.CollectorSkipped {
		t.Fatalf("unexpected validation result: %+v", result)
	}
}

func TestValidatorRejectsInvalidYAML(t *testing.T) {
	result := NewValidator("").Validate(context.Background(), "service:\n  pipelines: [\n")
	if result.Valid || result.YAMLValid || result.Error == "" {
		t.Fatalf("expected invalid YAML result, got: %+v", result)
	}
}

func TestValidatorRejectsEmptyContent(t *testing.T) {
	result := NewValidator("").Validate(context.Background(), "   \n")
	if result.Valid || result.Error == "" {
		t.Fatalf("expected empty content to fail, got: %+v", result)
	}
}

func TestValidatorRejectsUndefinedPipelineComponentWithoutCollectorBinary(t *testing.T) {
	content := `receivers:
  otlp:
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [missing]
`
	result := NewValidator("").Validate(context.Background(), content)
	if result.Valid {
		t.Fatal("configuration with undefined exporter unexpectedly valid")
	}
	if !strings.Contains(result.Error, `undefined exporter "missing"`) {
		t.Fatalf("error = %q", result.Error)
	}
}

func TestValidatorReturnsPipelineWarnings(t *testing.T) {
	result := NewValidator("").Validate(context.Background(), "receivers:\n  otlp:\nservice:\n  pipelines: {}\n")
	if !result.Valid {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != `receiver "otlp" is defined but unused` {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}
