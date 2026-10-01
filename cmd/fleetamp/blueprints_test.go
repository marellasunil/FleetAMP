package main

import (
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"gopkg.in/yaml.v3"
)

func generatedPipelines(t *testing.T, content string) map[string]any {
	t.Helper()
	var document struct {
		Service struct {
			Pipelines map[string]any `yaml:"pipelines"`
		} `yaml:"service"`
	}
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		t.Fatal(err)
	}
	return document.Service.Pipelines
}

func testPattern(name, platform, receiverID, config string, signals []string) *blueprints.Pattern {
	return blueprints.NewPattern(name, "", platform, receiverID, config, signals)
}

func testSafetyBlocks() []*blueprints.Block {
	return []*blueprints.Block{
		blueprints.NewBlock("Memory limiter", "", "processors", "memory_limiter", "check_interval: 1s\nlimit_mib: 512", []string{"metrics", "traces", "logs"}, []string{"any"}, true, true),
		blueprints.NewBlock("Batch", "", "processors", "batch", "timeout: 5s", []string{"metrics", "traces", "logs"}, []string{"any"}, true, true),
	}
}

func TestGenerateBlueprintYAMLOTLP(t *testing.T) {
	destination := blueprints.NewDestinationProfile("OTLP", "Production", "otlphttp/prod", "endpoint: https://example.invalid/otlp")
	content, err := generateBlueprintYAML(testPattern("OTLP service", "application", "otlp", "protocols:\n  grpc: {}\n  http: {}", []string{"metrics", "traces", "logs"}), []string{"metrics", "traces"}, testSafetyBlocks(), destination)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"otlphttp/prod:", "metrics:", "traces:", "memory_limiter", "batch"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("generated configuration does not contain %q:\n%s", expected, content)
		}
	}
	if _, exists := generatedPipelines(t, content)["logs"]; exists {
		t.Fatalf("unselected logs pipeline was generated:\n%s", content)
	}
	if result := configs.NewValidator("").Validate(t.Context(), content); !result.Valid {
		t.Fatalf("generated configuration did not validate: %s", result.Error)
	}
}

func TestGenerateHostBlueprintForcesMetrics(t *testing.T) {
	destination := blueprints.NewDestinationProfile("OTLP", "Development", "otlp/dev", "endpoint: http://gateway:4317")
	content, err := generateBlueprintYAML(testPattern("Host observability", "linux", "hostmetrics", "collection_interval: 30s\nscrapers:\n  cpu: {}\n  memory: {}", []string{"metrics"}), []string{"logs"}, testSafetyBlocks(), destination)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "hostmetrics:") || !strings.Contains(content, "metrics:") {
		t.Fatalf("host metrics pipeline missing:\n%s", content)
	}
	if _, exists := generatedPipelines(t, content)["logs"]; exists {
		t.Fatalf("host Blueprint must not generate a logs pipeline:\n%s", content)
	}
}

func TestGenerateBlueprintRequiresSignal(t *testing.T) {
	destination := blueprints.NewDestinationProfile("OTLP", "Development", "otlp/dev", "endpoint: http://gateway:4317")
	if _, err := generateBlueprintYAML(testPattern("OTLP service", "application", "otlp", "protocols:\n  grpc: {}", []string{"metrics", "traces", "logs"}), nil, testSafetyBlocks(), destination); err == nil {
		t.Fatal("expected an empty signal selection to fail")
	}
}

func TestDestinationConfigEncryption(t *testing.T) {
	pepper := []byte("0123456789abcdef0123456789abcdef")
	plaintext := "endpoint: https://example.invalid/otlp\ntls:\n  insecure: false"
	encrypted, err := encryptDestinationConfig(pepper, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == plaintext || strings.Contains(encrypted, "example.invalid") {
		t.Fatal("destination configuration was stored in clear text")
	}
	decrypted, err := decryptDestinationConfig(pepper, encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != plaintext {
		t.Fatalf("decrypted destination configuration differs: %q", decrypted)
	}
	if _, err := decryptDestinationConfig([]byte("different-pepper-material-00000000"), encrypted); err == nil {
		t.Fatal("destination configuration decrypted with a different server pepper")
	}
}

func TestGenerateBlueprintIncludesAdminBlock(t *testing.T) {
	destination := blueprints.NewDestinationProfile("OTLP", "Production", "otlphttp/prod", "endpoint: https://example.invalid/otlp")
	pattern := testPattern("OTLP service", "application", "otlp", "protocols:\n  grpc: {}", []string{"metrics"})
	memory := blueprints.NewBlock("Memory limiter", "", "processors", "memory_limiter", "check_interval: 1s\nlimit_mib: 768", []string{"metrics"}, []string{"application"}, false, true)
	content, err := generateBlueprintYAML(pattern, []string{"metrics"}, []*blueprints.Block{memory}, destination)
	if err != nil { t.Fatal(err) }
	if !strings.Contains(content, "memory_limiter:") || !strings.Contains(content, "limit_mib: 768") {
		t.Fatalf("admin block missing from generated Blueprint:\n%s", content)
	}
}

func TestBlueprintPageProvidesGuidedVisualJourney(t *testing.T) {
	for _, expected := range []string{
		"Observability goal",
		"Platform",
		"Technology",
		"Instrumentation method",
		"Recommended design",
		"Telemetry flow preview",
		"Validate and save version",
		"Validate, save & request approval",
		"data-pattern-platform",
		"data-implementation-steps",
		"data-flow-destination",
	} {
		if !strings.Contains(blueprintsHTML, expected) {
			t.Fatalf("guided Blueprint page is missing %q", expected)
		}
	}
}

func TestBlueprintJourneyRequiresCompatibleContext(t *testing.T) {
	if !strings.Contains(blueprintsHTML, `name="goal" required`) ||
		!strings.Contains(blueprintsHTML, `name="platform" required`) ||
		!strings.Contains(blueprintsHTML, `name="technology" required`) {
		t.Fatal("Blueprint context fields are not required")
	}
}
