package main

import (
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
)

func TestAdoptCollectorPatternReplacesOnlyMatchedReceiver(t *testing.T) {
	pattern := blueprints.NewPattern("Linux baseline", "", "linux", "hostmetrics", "collection_interval: 30s\nscrapers:\n  cpu: {}\n  memory: {}", []string{"metrics"})
	input := `receivers:
  hostmetrics/custom:
    collection_interval: 60s
    scrapers:
      cpu: {}
  filelog:
    include: [/var/log/app.log]
processors:
  batch: {}
exporters:
  debug: {}
service:
  pipelines:
    metrics:
      receivers: [hostmetrics/custom]
      processors: [batch]
      exporters: [debug]
`
	output, changes, err := adoptCollectorPattern(input, pattern)
	if err != nil {
		t.Fatalf("adopt Pattern: %v", err)
	}
	for _, preserved := range []string{"hostmetrics/custom:", "collection_interval: 30s", "memory: {}", "filelog:", "batch:", "debug:", "receivers: [hostmetrics/custom]"} {
		if !strings.Contains(output, preserved) {
			t.Fatalf("output did not preserve/apply %q:\n%s", preserved, output)
		}
	}
	if strings.Contains(output, "collection_interval: 60s") {
		t.Fatalf("old Pattern-owned receiver setting remained:\n%s", output)
	}
	if len(changes) != 1 || changes[0].Category != "Pattern" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestAdoptCollectorPatternIsIdempotent(t *testing.T) {
	pattern := blueprints.NewPattern("OTLP", "", "any", "otlp", "protocols:\n  grpc: {}", []string{"traces"})
	input := `receivers:
  otlp:
    protocols:
      grpc: {}
service:
  pipelines:
    traces:
      receivers: [otlp]
`
	first, _, err := adoptCollectorPattern(input, pattern)
	if err != nil {
		t.Fatalf("first adoption: %v", err)
	}
	second, changes, err := adoptCollectorPattern(first, pattern)
	if err != nil {
		t.Fatalf("second adoption: %v", err)
	}
	if first != second || len(changes) != 0 {
		t.Fatalf("adoption is not idempotent; changes=%#v\nfirst:\n%s\nsecond:\n%s", changes, first, second)
	}
}

func TestAdoptCollectorPatternRejectsMissingOrDisabledPattern(t *testing.T) {
	input := "receivers:\n  otlp: {}\nservice:\n  pipelines:\n    traces:\n      receivers: [otlp]\n"
	missing := blueprints.NewPattern("Host", "", "linux", "hostmetrics", "", []string{"metrics"})
	if _, _, err := adoptCollectorPattern(input, missing); err == nil {
		t.Fatal("expected missing receiver to be rejected")
	}
	disabled := blueprints.NewPattern("OTLP", "", "any", "otlp", "", []string{"traces"})
	disabled.Enabled = false
	if _, _, err := adoptCollectorPattern(input, disabled); err == nil {
		t.Fatal("expected disabled Pattern to be rejected")
	}
}

func TestAdoptCollectorPatternChangesOnlyOneAmbiguousReceiver(t *testing.T) {
	pattern := blueprints.NewPattern("OTLP", "", "any", "otlp", "protocols:\n  grpc: {}", []string{"traces"})
	input := `receivers:
  otlp/z: {}
  otlp/a: {}
service:
  pipelines:
    traces:
      receivers: [otlp/a, otlp/z]
`
	output, changes, err := adoptCollectorPattern(input, pattern)
	if err != nil {
		t.Fatalf("adopt Pattern: %v", err)
	}
	if !strings.Contains(output, "otlp/a:\n    protocols:") || !strings.Contains(output, "otlp/z: {}") {
		t.Fatalf("expected only deterministic first receiver to change:\n%s", output)
	}
	if len(changes) != 2 || changes[1].Category != "Scope" {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestMigrationProposalHashBindsPatternAndContent(t *testing.T) {
	baseline := migrationProposalHash("receivers: {}\n", "pattern-a")
	if baseline == migrationProposalHash("receivers: {otlp: {}}\n", "pattern-a") {
		t.Fatal("proposal hash did not bind content")
	}
	if baseline == migrationProposalHash("receivers: {}\n", "pattern-b") {
		t.Fatal("proposal hash did not bind Pattern identity")
	}
}
