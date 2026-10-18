package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const BuildSourceAcquired = "source-acquired"

type BuildSourceAcquisition struct {
	ID                 string    `json:"id"`
	SourceResolutionID string    `json:"source_resolution_id"`
	ExecutionID        string    `json:"execution_id"`
	BuildPlanHash      string    `json:"build_plan_hash"`
	ArchiveSHA256      string    `json:"archive_sha256"`
	ArchiveSize        int64     `json:"archive_size"`
	LocalReference     string    `json:"local_reference"`
	Status             string    `json:"status"`
	EvidenceHash       string    `json:"evidence_hash"`
	AcquiredBy         string    `json:"acquired_by"`
	AcquiredAt         time.Time `json:"acquired_at"`
}

func NewBuildSourceAcquisition(source *BuildSourceResolution, archiveSHA string, archiveSize int64, localReference, actor string, now time.Time) (*BuildSourceAcquisition, error) {
	if source == nil {
		return nil, errors.New("source resolution is required")
	}
	archiveSHA = strings.ToLower(strings.TrimSpace(archiveSHA))
	localReference = strings.TrimSpace(localReference)
	actor = strings.TrimSpace(actor)
	if source.Status != BuildSourceResolved {
		return nil, errors.New("source resolution is not ready for acquisition")
	}
	if len(archiveSHA) != 64 || !isHex(archiveSHA) {
		return nil, errors.New("source archive SHA-256 is invalid")
	}
	if archiveSize <= 0 {
		return nil, errors.New("source archive must not be empty")
	}
	if !strings.HasPrefix(localReference, "source-cache://") || strings.Contains(localReference, "..") {
		return nil, errors.New("source cache reference is invalid")
	}
	if actor == "" {
		return nil, errors.New("source acquisition actor is required")
	}
	canonical, _ := json.Marshal(struct {
		ResolutionID, ExecutionID, PlanHash, ArchiveSHA, LocalReference string
		ArchiveSize                                                     int64
	}{source.ID, source.ExecutionID, source.BuildPlanHash, archiveSHA, localReference, archiveSize})
	sum := sha256.Sum256(canonical)
	evidenceHash := hex.EncodeToString(sum[:])
	return &BuildSourceAcquisition{ID: "build-acquisition-" + evidenceHash[:16], SourceResolutionID: source.ID, ExecutionID: source.ExecutionID, BuildPlanHash: source.BuildPlanHash, ArchiveSHA256: archiveSHA, ArchiveSize: archiveSize, LocalReference: localReference, Status: BuildSourceAcquired, EvidenceHash: evidenceHash, AcquiredBy: actor, AcquiredAt: now.UTC()}, nil
}
