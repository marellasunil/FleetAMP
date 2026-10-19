package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const BuildArtifactGenerated = "artifact-generated"

type OCBRunResult struct {
	Runtime, BuilderImage, ArtifactReference, ArtifactSHA256 string
	ArtifactSize                                             int64
}

type OCBRunner interface {
	Build(context.Context, *BuildSourceInspection) (*OCBRunResult, error)
}

type BuildArtifact struct {
	ID, InspectionID, ExecutionID, BuildPlanHash string
	ManifestSHA256, Runtime, BuilderImage        string
	ArtifactReference, ArtifactSHA256            string
	ArtifactSize                                 int64
	Status, EvidenceHash, BuiltBy                string
	BuiltAt                                      time.Time
}

func NewBuildArtifact(inspection *BuildSourceInspection, result *OCBRunResult, actor string, now time.Time) (*BuildArtifact, error) {
	if inspection == nil || result == nil {
		return nil, errors.New("source inspection and OCB result are required")
	}
	if inspection.Status != BuildSourceInspected {
		return nil, errors.New("source inspection is not ready for build")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("build actor is required")
	}
	if result.Runtime != "docker" && result.Runtime != "podman" {
		return nil, errors.New("unsupported sandbox runtime")
	}
	if !validDigestReference(result.BuilderImage) {
		return nil, errors.New("builder image must be pinned by SHA-256 digest")
	}
	if len(result.ArtifactSHA256) != 64 || !isHex(result.ArtifactSHA256) || result.ArtifactSize <= 0 {
		return nil, errors.New("generated artifact evidence is invalid")
	}
	if !strings.HasPrefix(result.ArtifactReference, "artifact-cache://") || strings.Contains(result.ArtifactReference, "..") {
		return nil, errors.New("generated artifact reference is invalid")
	}
	canonical, _ := json.Marshal(struct {
		InspectionID, ExecutionID, PlanHash, ManifestSHA, Runtime, Image, Reference, ArtifactSHA string
		ArtifactSize                                                                             int64
	}{inspection.ID, inspection.ExecutionID, inspection.BuildPlanHash, inspection.ManifestSHA256, result.Runtime, result.BuilderImage, result.ArtifactReference, result.ArtifactSHA256, result.ArtifactSize})
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &BuildArtifact{ID: "build-artifact-" + evidence[:16], InspectionID: inspection.ID, ExecutionID: inspection.ExecutionID, BuildPlanHash: inspection.BuildPlanHash, ManifestSHA256: inspection.ManifestSHA256, Runtime: result.Runtime, BuilderImage: result.BuilderImage, ArtifactReference: result.ArtifactReference, ArtifactSHA256: result.ArtifactSHA256, ArtifactSize: result.ArtifactSize, Status: BuildArtifactGenerated, EvidenceHash: evidence, BuiltBy: actor, BuiltAt: now.UTC()}, nil
}

func validDigestReference(value string) bool {
	parts := strings.Split(strings.TrimSpace(value), "@sha256:")
	return len(parts) == 2 && parts[0] != "" && len(parts[1]) == 64 && isHex(parts[1])
}
