package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestArtifactPromotionFourEyesAndExactEvidence(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	artifact := &BuildArtifact{ID: "artifact-1", BuildPlanHash: strings.Repeat("a", 64), ArtifactSHA256: strings.Repeat("b", 64), Status: BuildArtifactGenerated}
	plan := &BuildPlan{PlanHash: artifact.BuildPlanHash, Manifest: BuildManifest{DestinationRef: "oci://registry.example/platform/collector:1.0.0", CredentialRef: "secret://registry/platform"}}
	promotion, err := NewArtifactPromotion(artifact, plan, "Release approved Collector", "requester", now)
	if err != nil {
		t.Fatal(err)
	}
	if promotion.DestinationRef != plan.Manifest.DestinationRef || promotion.ArtifactSHA256 != artifact.ArtifactSHA256 {
		t.Fatalf("promotion did not freeze exact evidence: %#v", promotion)
	}
	if _, err := NewArtifactPromotionReview(promotion, ArtifactPromotionApproved, "looks good", "requester", now.Add(time.Minute)); err == nil {
		t.Fatal("requester reviewed their own promotion")
	}
	review, err := NewArtifactPromotionReview(promotion, ArtifactPromotionApproved, "verified evidence", "reviewer", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	publication, err := NewArtifactPublication(promotion, review, "publisher", now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if publication.Status != ArtifactPublicationQueued || publication.ArtifactSHA256 != artifact.ArtifactSHA256 {
		t.Fatalf("unexpected publication: %#v", publication)
	}
}

func TestRejectedPromotionCannotBeQueued(t *testing.T) {
	now := time.Now().UTC()
	artifact := &BuildArtifact{ID: "artifact-1", BuildPlanHash: strings.Repeat("a", 64), ArtifactSHA256: strings.Repeat("b", 64), Status: BuildArtifactGenerated}
	plan := &BuildPlan{PlanHash: artifact.BuildPlanHash, Manifest: BuildManifest{DestinationRef: "oci://registry.example/collector:1", CredentialRef: "secret://registry"}}
	promotion, _ := NewArtifactPromotion(artifact, plan, "release", "alice", now)
	review, _ := NewArtifactPromotionReview(promotion, ArtifactPromotionRejected, "not ready", "bob", now)
	if _, err := NewArtifactPublication(promotion, review, "bob", now); err == nil {
		t.Fatal("rejected promotion was queued")
	}
}
