package configs

import (
	"context"
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
