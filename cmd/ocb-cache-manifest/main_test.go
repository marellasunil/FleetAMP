package main

import (
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

func TestRenderModuleFileUsesReviewedCatalogDeterministically(t *testing.T) {
	one := renderModuleFile("0.140.0", distributions.ReviewedOCBModules())
	two := renderModuleFile("0.140.0", distributions.ReviewedOCBModules())
	if one != two {
		t.Fatal("offline cache module manifest is not deterministic")
	}
	for _, module := range distributions.ReviewedOCBModules() {
		if !strings.Contains(one, module+" v0.140.0") {
			t.Fatalf("reviewed module %q missing from manifest", module)
		}
	}
	if !strings.Contains(one, "go.opentelemetry.io/collector/cmd/builder v0.140.0") {
		t.Fatal("version-matched OCB module is missing")
	}
}

func TestCollectorVersionPatternRejectsMutableOrMalformedVersions(t *testing.T) {
	for _, value := range []string{"latest", "v0.140.0", "0.140", "0.140.0-dev", "0.140.0;bad"} {
		if collectorVersionPattern.MatchString(value) {
			t.Fatalf("unexpected accepted version %q", value)
		}
	}
}
