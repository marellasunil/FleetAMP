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

func TestBlueprintPageOffersCommonStarters(t *testing.T) {
	for _, expected := range []string{
		"Starter catalog",
		"Choose what you want to observe",
		"Use Blueprint",
		"Generated Collector configuration",
		"Validate &amp; save version",
		`href="/instrumentation"`,
	} {
		if !strings.Contains(blueprintsHTML, expected) {
			t.Fatalf("Blueprint page is missing %q", expected)
		}
	}
}

func TestCommonBlueprintCatalog(t *testing.T) {
	starters := blueprints.CommonStarters()
	if len(starters) != 6 { t.Fatalf("got %d common Blueprints, want 6", len(starters)) }
	seen := map[string]bool{}
	for _, starter := range starters {
		if starter.ID == "" || starter.Name == "" || starter.Pattern == nil { t.Fatalf("incomplete starter: %#v", starter) }
		if seen[starter.ID] { t.Fatalf("duplicate starter ID %q", starter.ID) }
		seen[starter.ID] = true
		if !blueprintApproachAllowed(starter.Goal, starter.Platform, starter.Method) {
			t.Fatalf("starter %q has incompatible approach %s/%s/%s", starter.ID, starter.Goal, starter.Platform, starter.Method)
		}
	}
}

func TestBlueprintApproachCompatibility(t *testing.T) {
	for _, test := range []struct{ goal, platform, method string; allowed bool }{
		{"apm", "linux", "auto-linux", true},
		{"apm", "linux", "ebpf", true},
		{"apm", "kubernetes", "operator", true},
		{"kubernetes", "kubernetes", "daemonset", true},
		{"kubernetes", "kubernetes", "deployment", true},
		{"apm", "kubernetes", "sidecar", true},
		{"apm", "linux", "operator", false},
		{"infrastructure", "linux", "sidecar", false},
	} {
		if got := blueprintApproachAllowed(test.goal, test.platform, test.method); got != test.allowed {
			t.Fatalf("blueprintApproachAllowed(%q,%q,%q)=%v, want %v", test.goal, test.platform, test.method, got, test.allowed)
		}
	}
}
