package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestNewDistributionReleaseBindsSuccessfulPublication(t *testing.T) {
	now := time.Unix(10, 0)
	d, err := New("Platform Collector", "0.140.0", "v0.140.0", "", "", ComponentSet{Receivers: []string{"otlp"}}, "receivers:\n  otlp: {}", []string{"service"}, []string{"receivers"}, "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewBuildPlan(d, []string{"oci-image"}, "oci://registry.example.test/platform/collector:0.140.0", "secret://registry/platform", "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	artifact := &BuildArtifact{ID: "artifact-1", BuildPlanHash: plan.PlanHash, ArtifactSHA256: strings.Repeat("a", 64), EvidenceHash: strings.Repeat("b", 64), Status: BuildArtifactGenerated}
	publication := &ArtifactPublication{ID: "publication-1", ArtifactID: artifact.ID, ArtifactSHA256: artifact.ArtifactSHA256, DestinationRef: plan.Manifest.DestinationRef, EvidenceHash: strings.Repeat("c", 64), Status: ArtifactPublicationQueued}
	execution := &ArtifactPublicationExecution{ID: "execution-1", PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, ArtifactID: artifact.ID, ArtifactSHA256: artifact.ArtifactSHA256, DestinationRef: publication.DestinationRef, Status: ArtifactPublicationSucceeded, EvidenceHash: strings.Repeat("d", 64)}
	release, err := NewDistributionRelease(d, plan, artifact, publication, execution, "release-admin", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if release.ArtifactURI != publication.DestinationRef || release.ArtifactSHA256 != artifact.ArtifactSHA256 || release.DistributionSpecHash != d.SpecHash {
		t.Fatalf("unexpected release: %#v", release)
	}
	execution.Status = ArtifactPublicationFailed
	if _, err := NewDistributionRelease(d, plan, artifact, publication, execution, "release-admin", now); err == nil {
		t.Fatal("failed publication accepted")
	}
}
