package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	ArtifactPromotionPending  = "pending"
	ArtifactPromotionApproved = "approved"
	ArtifactPromotionRejected = "rejected"
	ArtifactPublicationQueued = "queued"
)

// ArtifactPromotion freezes the exact build artifact and approved build-plan
// destination that an administrator wants to publish.
type ArtifactPromotion struct {
	ID, ArtifactID, ArtifactSHA256, BuildPlanHash string
	DestinationRef, CredentialRef                 string
	Reason, RequestedBy, Status, EvidenceHash     string
	RequestedAt                                   time.Time
}

func NewArtifactPromotion(artifact *BuildArtifact, plan *BuildPlan, reason, actor string, now time.Time) (*ArtifactPromotion, error) {
	if artifact == nil || plan == nil || artifact.BuildPlanHash != plan.PlanHash {
		return nil, errors.New("matching build artifact and immutable build plan are required")
	}
	if artifact.Status != BuildArtifactGenerated || len(artifact.ArtifactSHA256) != 64 || !isHex(artifact.ArtifactSHA256) {
		return nil, errors.New("verified generated artifact is required")
	}
	reason, actor = strings.TrimSpace(reason), strings.TrimSpace(actor)
	if reason == "" || actor == "" {
		return nil, errors.New("promotion reason and requester are required")
	}
	if len(reason) > 500 {
		return nil, errors.New("promotion reason must not exceed 500 characters")
	}
	if strings.TrimSpace(plan.Manifest.DestinationRef) == "" || strings.TrimSpace(plan.Manifest.CredentialRef) == "" {
		return nil, errors.New("build plan destination and credential reference are required")
	}
	payload := struct {
		ArtifactID, ArtifactSHA256, BuildPlanHash, DestinationRef, CredentialRef string
		Reason, Actor, Status                                                    string
		At                                                                       time.Time
	}{artifact.ID, artifact.ArtifactSHA256, plan.PlanHash, plan.Manifest.DestinationRef, plan.Manifest.CredentialRef, reason, actor, ArtifactPromotionPending, now.UTC()}
	evidence := hashPromotionEvidence(payload)
	return &ArtifactPromotion{ID: "artifact-promotion-" + evidence[:16], ArtifactID: artifact.ID, ArtifactSHA256: artifact.ArtifactSHA256, BuildPlanHash: plan.PlanHash, DestinationRef: plan.Manifest.DestinationRef, CredentialRef: plan.Manifest.CredentialRef, Reason: reason, RequestedBy: actor, Status: ArtifactPromotionPending, EvidenceHash: evidence, RequestedAt: now.UTC()}, nil
}

// ArtifactPromotionReview is an immutable four-eyes decision over the exact
// proposal evidence. It never mutates the original proposal.
type ArtifactPromotionReview struct {
	ID, PromotionID, PromotionEvidenceHash, ArtifactSHA256 string
	Decision, Reason, ReviewedBy, EvidenceHash             string
	ReviewedAt                                             time.Time
}

func NewArtifactPromotionReview(promotion *ArtifactPromotion, decision, reason, actor string, now time.Time) (*ArtifactPromotionReview, error) {
	if promotion == nil || promotion.Status != ArtifactPromotionPending {
		return nil, errors.New("pending artifact promotion is required")
	}
	decision, reason, actor = strings.TrimSpace(decision), strings.TrimSpace(reason), strings.TrimSpace(actor)
	if decision != ArtifactPromotionApproved && decision != ArtifactPromotionRejected {
		return nil, errors.New("promotion decision must be approved or rejected")
	}
	if reason == "" || actor == "" {
		return nil, errors.New("review reason and reviewer are required")
	}
	if actor == promotion.RequestedBy {
		return nil, errors.New("promotion requester cannot review their own proposal")
	}
	if len(reason) > 500 {
		return nil, errors.New("review reason must not exceed 500 characters")
	}
	payload := struct {
		PromotionID, PromotionEvidenceHash, ArtifactSHA256, Decision, Reason, Actor string
		At                                                                          time.Time
	}{promotion.ID, promotion.EvidenceHash, promotion.ArtifactSHA256, decision, reason, actor, now.UTC()}
	evidence := hashPromotionEvidence(payload)
	return &ArtifactPromotionReview{ID: "artifact-promotion-review-" + evidence[:16], PromotionID: promotion.ID, PromotionEvidenceHash: promotion.EvidenceHash, ArtifactSHA256: promotion.ArtifactSHA256, Decision: decision, Reason: reason, ReviewedBy: actor, EvidenceHash: evidence, ReviewedAt: now.UTC()}, nil
}

// ArtifactPublication is an immutable outbox record. A later opt-in registry
// worker may consume it; creating it performs no network or registry write.
type ArtifactPublication struct {
	ID, PromotionID, ReviewID, ArtifactID, ArtifactSHA256 string
	DestinationRef, CredentialRef, Status, EvidenceHash   string
	QueuedBy                                              string
	QueuedAt                                              time.Time
}

func NewArtifactPublication(promotion *ArtifactPromotion, review *ArtifactPromotionReview, actor string, now time.Time) (*ArtifactPublication, error) {
	if promotion == nil || review == nil || review.PromotionID != promotion.ID || review.PromotionEvidenceHash != promotion.EvidenceHash || review.ArtifactSHA256 != promotion.ArtifactSHA256 {
		return nil, errors.New("review does not match exact promotion evidence")
	}
	if review.Decision != ArtifactPromotionApproved {
		return nil, errors.New("approved promotion review is required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("publication queue actor is required")
	}
	payload := struct {
		PromotionID, ReviewID, ArtifactID, ArtifactSHA256, DestinationRef, CredentialRef, Status, Actor string
		At                                                                                              time.Time
	}{promotion.ID, review.ID, promotion.ArtifactID, promotion.ArtifactSHA256, promotion.DestinationRef, promotion.CredentialRef, ArtifactPublicationQueued, actor, now.UTC()}
	evidence := hashPromotionEvidence(payload)
	return &ArtifactPublication{ID: "artifact-publication-" + evidence[:16], PromotionID: promotion.ID, ReviewID: review.ID, ArtifactID: promotion.ArtifactID, ArtifactSHA256: promotion.ArtifactSHA256, DestinationRef: promotion.DestinationRef, CredentialRef: promotion.CredentialRef, Status: ArtifactPublicationQueued, EvidenceHash: evidence, QueuedBy: actor, QueuedAt: now.UTC()}, nil
}

func hashPromotionEvidence(value any) string {
	canonical, _ := json.Marshal(value)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}
