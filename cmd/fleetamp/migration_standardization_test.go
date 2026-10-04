package main

import (
	"strings"
	"testing"
)

func TestStandardizeImportedConfigurationUsesCanonicalOrderAndPreservesUnknownSections(t *testing.T) {
	standardized, changes, err := standardizeImportedConfiguration(`
service:
  pipelines:
    traces:
      exporters: [debug]
      receivers: [otlp]
custom_section:
  enabled: true
exporters:
  debug: {}
receivers:
  zipkin: {}
  otlp: {}
`, false)
	if err != nil {
		t.Fatal(err)
	}
	receivers := strings.Index(standardized, "receivers:")
	exporters := strings.Index(standardized, "exporters:")
	service := strings.Index(standardized, "service:")
	custom := strings.Index(standardized, "custom_section:")
	if !(receivers < exporters && exporters < service && service < custom) {
		t.Fatalf("sections are not canonical:\n%s", standardized)
	}
	if strings.Index(standardized, "  otlp:") > strings.Index(standardized, "  zipkin:") {
		t.Fatalf("receiver names are not sorted:\n%s", standardized)
	}
	if len(changes) == 0 || changes[0].Category != "Layout" {
		t.Fatalf("changes=%v", changes)
	}
}

func TestStandardizeImportedConfigurationAppliesSafeBaselineWithoutReplacingExistingProcessors(t *testing.T) {
	standardized, changes, err := standardizeImportedConfiguration(`
receivers:
  otlp: {}
processors:
  attributes/team: {}
exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [attributes/team]
      exporters: [debug]
`, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"memory_limiter:",
		"limit_mib: 512",
		"batch: {}",
		"- memory_limiter",
		"- attributes/team",
		"- batch",
	} {
		if !strings.Contains(standardized, expected) {
			t.Fatalf("standardized YAML is missing %q:\n%s", expected, standardized)
		}
	}
	if len(changes) < 4 {
		t.Fatalf("expected layout, processor and pipeline changes; got %v", changes)
	}
}

func TestStandardizeImportedConfigurationRecognizesNamedBaselineComponents(t *testing.T) {
	standardized, _, err := standardizeImportedConfiguration(`
processors:
  memory_limiter/standard: {}
  batch/production: {}
service:
  pipelines:
    metrics:
      processors: [memory_limiter/standard, batch/production]
`, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(standardized, "\n  memory_limiter:\n") || strings.Contains(standardized, "\n  batch: {}") {
		t.Fatalf("duplicate baseline components were added:\n%s", standardized)
	}
}

func TestStandardizeImportedConfigurationIsIdempotent(t *testing.T) {
	first, _, err := standardizeImportedConfiguration(`
exporters:
  debug: {}
receivers:
  otlp: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
`, true)
	if err != nil {
		t.Fatal(err)
	}
	second, changes, err := standardizeImportedConfiguration(first, true)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("standardization is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if len(changes) != 0 {
		t.Fatalf("second standardization unexpectedly changed the configuration: %v", changes)
	}
}

func TestStandardizeImportedConfigurationRejectsMultipleDocuments(t *testing.T) {
	_, _, err := standardizeImportedConfiguration("receivers: {}\n---\nexporters: {}\n", false)
	if err == nil || !strings.Contains(err.Error(), "exactly one document") {
		t.Fatalf("error=%v", err)
	}
}

func TestStandardizeImportedConfigurationRequiresPipelinesForBaseline(t *testing.T) {
	_, _, err := standardizeImportedConfiguration("receivers: {}\n", true)
	if err == nil || !strings.Contains(err.Error(), "service.pipelines") {
		t.Fatalf("error=%v", err)
	}
}
