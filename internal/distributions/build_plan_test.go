package distributions

import (
	"testing"
	"time"
)

func TestBuildPlanBindsImmutableDistributionEvidence(t *testing.T) {
	d, err := New("Platform Collector", "0.140.0", "v0.140.0", "", "", ComponentSet{Receivers: []string{"otlp"}}, "receivers:\n  otlp: {}", nil, []string{"receivers"}, "admin", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	one, err := NewBuildPlan(d, []string{"binary-archive", "oci-image"}, "oci://registry.example/platform/collector:0.140.0", "secret://registry/platform", "admin", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewBuildPlan(d, []string{"oci-image", "binary-archive"}, "oci://registry.example/platform/collector:0.140.0", "secret://registry/platform", "other", time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != two.ID || one.PlanHash != two.PlanHash {
		t.Fatalf("plan evidence is not stable: %#v %#v", one, two)
	}
	if one.Manifest.DistributionSpecHash != d.SpecHash || one.Manifest.BaselineHash != d.BaselineHash {
		t.Fatal("distribution evidence was not frozen")
	}
}

func TestBuildPlanRejectsUnsafeReferences(t *testing.T) {
	d := &Distribution{ID: "dist", SpecHash: "hash", BaselineHash: "baseline", CollectorVersion: "0.140.0", SourceRepository: OfficialContribRepository, SourceRef: "v0.140.0"}
	if _, err := NewBuildPlan(d, []string{"shell-command"}, "oci://registry/x", "secret://x", "admin", time.Now()); err == nil {
		t.Fatal("expected unsupported output rejection")
	}
	if _, err := NewBuildPlan(d, []string{"oci-image"}, "https://user:password@registry/x", "password", "admin", time.Now()); err == nil {
		t.Fatal("expected unsafe reference rejection")
	}
}
