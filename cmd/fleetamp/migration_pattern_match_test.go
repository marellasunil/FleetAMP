package main

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
)

func TestMatchCollectorPatternsRanksLinuxHostBaseline(t *testing.T) {
	content := `receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu: {}
      memory: {}
      disk: {}
      filesystem: {}
      load: {}
      network: {}
      processes: {}
service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
`
	matches, err := matchCollectorPatterns(content, starterPatterns())
	if err != nil {
		t.Fatalf("match patterns: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected a matching Pattern")
	}
	if matches[0].Name != "Linux host baseline" {
		t.Fatalf("top match = %q, want Linux host baseline", matches[0].Name)
	}
	if matches[0].Score < 90 || !matches[0].Recommended || matches[0].Confidence != "High" {
		t.Fatalf("unexpected top match: %#v", matches[0])
	}
}

func TestMatchCollectorPatternsDistinguishesOTLPGateway(t *testing.T) {
	content := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
service:
  pipelines:
    metrics:
      receivers: [otlp]
    traces:
      receivers: [otlp]
    logs:
      receivers: [otlp]
`
	matches, err := matchCollectorPatterns(content, starterPatterns())
	if err != nil {
		t.Fatalf("match patterns: %v", err)
	}
	scores := map[string]int{}
	for _, match := range matches {
		scores[match.Name] = match.Score
	}
	if scores["Central OTLP gateway"] <= scores["OTLP application service"] {
		t.Fatalf("gateway score %d must exceed application score %d", scores["Central OTLP gateway"], scores["OTLP application service"])
	}
	if len(matches) == 0 || matches[0].Name != "Central OTLP gateway" {
		t.Fatalf("top match = %#v, want Central OTLP gateway", matches)
	}
}

func TestMatchCollectorPatternsLeavesUnknownConfigurationCustom(t *testing.T) {
	content := `receivers:
  syslog:
    tcp:
      listen_address: 0.0.0.0:54526
service:
  pipelines:
    logs:
      receivers: [syslog]
`
	matches, err := matchCollectorPatterns(content, starterPatterns())
	if err != nil {
		t.Fatalf("match patterns: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %#v, want none", matches)
	}
}

func TestMatchCollectorPatternsIgnoresDisabledPatterns(t *testing.T) {
	pattern := blueprints.NewPattern("Disabled", "", "linux", "hostmetrics", "", []string{"metrics"})
	pattern.Enabled = false
	content := `receivers:
  hostmetrics: {}
service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
`
	matches, err := matchCollectorPatterns(content, []*blueprints.Pattern{pattern})
	if err != nil {
		t.Fatalf("match patterns: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches = %#v, want disabled Pattern excluded", matches)
	}
}

func TestValidPatternSelectionRequiresOfferedMatch(t *testing.T) {
	matches := []migrationPatternMatch{{ID: "approved"}}
	if !validPatternSelection("approved", matches) {
		t.Fatal("expected offered Pattern to be valid")
	}
	if !validPatternSelection("custom", matches) {
		t.Fatal("expected explicit custom selection to be valid")
	}
	if validPatternSelection("unoffered", matches) {
		t.Fatal("unoffered Pattern must not be valid")
	}
}

func TestMatchCollectorPatternsRejectsMalformedConfiguration(t *testing.T) {
	if _, err := matchCollectorPatterns("receivers: [", starterPatterns()); err == nil {
		t.Fatal("expected malformed YAML to be rejected")
	}
}

func starterPatterns() []*blueprints.Pattern {
	starters := blueprints.CommonStarters()
	patterns := make([]*blueprints.Pattern, 0, len(starters))
	for _, starter := range starters {
		patterns = append(patterns, starter.Pattern)
	}
	return patterns
}
