package configs

import (
	"strings"
	"testing"
)

func TestSplitAndComposeConfigurationSections(t *testing.T) {
	original := `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch: {}
exporters:
  debug: {}
custom_top_level:
  preserved: true
service:
  extensions: [health_check]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
  telemetry:
    logs:
      level: info
  custom_service_key: preserved
`
	sections, err := SplitConfigurationSections(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		key, contains string
	}{
		{SectionReceivers, "otlp:"},
		{SectionExporters, "debug:"},
		{SectionServicePipelines, "traces:"},
		{SectionServiceExtensions, "health_check"},
		{SectionTelemetry, "level: info"},
	} {
		if !strings.Contains(sections[check.key], check.contains) {
			t.Fatalf("%s fragment=%q, want %q", check.key, sections[check.key], check.contains)
		}
	}
	sections[SectionProcessors] = "memory_limiter:\n  limit_mib: 256\n"
	composed, err := ComposeConfigurationSections(original, sections)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"memory_limiter:", "limit_mib: 256", "custom_top_level:",
		"custom_service_key: preserved", "pipelines:", "telemetry:",
	} {
		if !strings.Contains(composed, expected) {
			t.Fatalf("composed configuration missing %q:\n%s", expected, composed)
		}
	}
	changed, err := ChangedConfigurationSections(original, composed)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != SectionProcessors {
		t.Fatalf("changed sections=%v, want [%s]", changed, SectionProcessors)
	}
}

func TestComposeDeletesEmptySection(t *testing.T) {
	composed, err := ComposeConfigurationSections(
		"exporters:\n  debug: {}\nservice:\n  extensions: [health_check]\n",
		map[string]string{SectionExporters: "", SectionServiceExtensions: ""},
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(composed, "exporters:") || strings.Contains(composed, "service:") {
		t.Fatalf("empty sections were not removed:\n%s", composed)
	}
}

func TestComposeRejectsInvalidSectionYAML(t *testing.T) {
	_, err := ComposeConfigurationSections("", map[string]string{
		SectionReceivers: "otlp: [",
	})
	if err == nil || !strings.Contains(err.Error(), "Receivers") {
		t.Fatalf("error=%v, want Receivers validation error", err)
	}
}

func TestComposeNormalizesLabelledSectionFragments(t *testing.T) {
	composed, err := ComposeConfigurationSections("", map[string]string{
		SectionReceivers:        "receivers:\n  otlp:\n    protocols:\n      grpc: {}\n",
		SectionProcessors:       "processors:\n  batch: {}\n",
		SectionExporters:        "exporters:\n  debug: {}\n",
		SectionServicePipelines: "service:\n  pipelines:\n    traces:\n      receivers: [otlp]\n      processors: [batch]\n      exporters: [debug]\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"receivers:\n  receivers:", "processors:\n  processors:", "exporters:\n  exporters:", "connectors:\n  service:"} {
		if strings.Contains(composed, forbidden) {
			t.Fatalf("composed configuration contains duplicated wrapper %q:\n%s", forbidden, composed)
		}
	}
	result := CompareDesiredEffective(composed, `receivers:
  otlp:
    protocols:
      grpc: {}
processors:
  batch: {}
exporters:
  debug: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`)
	if result.Status != DriftInSync {
		t.Fatalf("normalized configuration should be in sync, got %#v\n%s", result, composed)
	}
}

func TestComposeRejectsSectionPlacedInWrongTab(t *testing.T) {
	_, err := ComposeConfigurationSections("", map[string]string{
		SectionConnectors: "service:\n  pipelines:\n    traces: {}\n",
	})
	if err == nil || !strings.Contains(err.Error(), "service belongs in the Service pipelines") {
		t.Fatalf("error=%v, want misplaced service section error", err)
	}
}
