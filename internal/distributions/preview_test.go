package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestDeploymentPreviewBindsDistributionGroupAndContent(t *testing.T) {
	d, err := New("Platform Collector", "0.140.0", "v0.140.0", "", "", ComponentSet{Receivers: []string{"otlp"}}, "receivers:\n  otlp: {}", nil, []string{"receivers"}, "admin", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	release := &DistributionRelease{ID: "release-1", DistributionID: d.ID, DistributionSpecHash: d.SpecHash, ArtifactURI: "oci://registry.example/collector:v1", ArtifactSHA256: strings.Repeat("a", 64), EvidenceHash: strings.Repeat("b", 64)}
	one, err := NewDeploymentPreview(d, release, "payments", "Payments", map[string]string{"receivers": "otlp: {}"}, "receivers:\n  otlp: {}", "{\"valid\":true}", "operator", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewDeploymentPreview(d, release, "payments", "Payments", map[string]string{"receivers": "otlp: {}"}, "receivers:\n  otlp: {}", "{\"valid\":true}", "operator", time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != two.ID || one.ContentHash != two.ContentHash {
		t.Fatalf("preview evidence is not stable: %#v %#v", one, two)
	}
	if one.DistributionSpecHash != d.SpecHash || one.ReleaseEvidenceHash != release.EvidenceHash || one.ArtifactSHA256 != release.ArtifactSHA256 {
		t.Fatal("distribution release evidence was not bound")
	}
	if _, err := NewDeploymentPreview(d, nil, "payments", "Payments", nil, "receivers: {}", "ok", "operator", time.Now()); err == nil {
		t.Fatal("unreleased distribution accepted")
	}
}
