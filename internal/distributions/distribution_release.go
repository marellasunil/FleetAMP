package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// DistributionRelease is immutable evidence that an administrator made an
// exact, successfully published Collector artifact available to consumers.
type DistributionRelease struct {
	ID, DistributionID, DistributionSpecHash, BuildPlanID, BuildPlanHash string
	ArtifactID, ArtifactEvidenceHash, ArtifactSHA256                     string
	PublicationID, PublicationEvidenceHash                               string
	ExecutionID, ExecutionEvidenceHash, ArtifactURI                      string
	ReleasedBy, EvidenceHash                                             string
	ReleasedAt                                                           time.Time
}

func NewDistributionRelease(distribution *Distribution, plan *BuildPlan, artifact *BuildArtifact, publication *ArtifactPublication, execution *ArtifactPublicationExecution, actor string, now time.Time) (*DistributionRelease, error) {
	if distribution == nil || plan == nil || artifact == nil || publication == nil || execution == nil {
		return nil, errors.New("distribution, plan, artifact, publication and execution are required")
	}
	if plan.DistributionID != distribution.ID || plan.Manifest.DistributionID != distribution.ID || plan.Manifest.DistributionSpecHash != distribution.SpecHash || artifact.BuildPlanHash != plan.PlanHash {
		return nil, errors.New("build evidence does not match the exact distribution specification")
	}
	if publication.ArtifactID != artifact.ID || publication.ArtifactSHA256 != artifact.ArtifactSHA256 || execution.PublicationID != publication.ID || execution.PublicationEvidenceHash != publication.EvidenceHash || execution.ArtifactID != artifact.ID || execution.ArtifactSHA256 != artifact.ArtifactSHA256 || execution.DestinationRef != publication.DestinationRef || execution.Status != ArtifactPublicationSucceeded {
		return nil, errors.New("successful publication evidence does not match the exact artifact")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" || !strings.HasPrefix(publication.DestinationRef, "oci://") {
		return nil, errors.New("release actor and OCI artifact destination are required")
	}
	payload := struct {
		DistributionID, DistributionSpecHash, BuildPlanID, BuildPlanHash string
		ArtifactID, ArtifactEvidenceHash, ArtifactSHA256                 string
		PublicationID, PublicationEvidenceHash                           string
		ExecutionID, ExecutionEvidenceHash, ArtifactURI, Actor           string
		At                                                               time.Time
	}{distribution.ID, distribution.SpecHash, plan.ID, plan.PlanHash, artifact.ID, artifact.EvidenceHash, artifact.ArtifactSHA256, publication.ID, publication.EvidenceHash, execution.ID, execution.EvidenceHash, publication.DestinationRef, actor, now.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &DistributionRelease{ID: "distribution-release-" + evidence[:16], DistributionID: distribution.ID, DistributionSpecHash: distribution.SpecHash, BuildPlanID: plan.ID, BuildPlanHash: plan.PlanHash, ArtifactID: artifact.ID, ArtifactEvidenceHash: artifact.EvidenceHash, ArtifactSHA256: artifact.ArtifactSHA256, PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, ExecutionID: execution.ID, ExecutionEvidenceHash: execution.EvidenceHash, ArtifactURI: publication.DestinationRef, ReleasedBy: actor, EvidenceHash: evidence, ReleasedAt: now.UTC()}, nil
}
