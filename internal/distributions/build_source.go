package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

const BuildSourceResolved = "source-resolved"

var officialReleaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

// BuildSourceResolution is an immutable, network-free source-fetch plan. A
// later downloader must verify the same repository, tag and evidence hash.
type BuildSourceResolution struct {
	ID             string    `json:"id"`
	ExecutionID    string    `json:"execution_id"`
	WorkerResultID string    `json:"worker_result_id"`
	BuildPlanHash  string    `json:"build_plan_hash"`
	Repository     string    `json:"repository"`
	SourceRef      string    `json:"source_ref"`
	ArchiveURL     string    `json:"archive_url"`
	Status         string    `json:"status"`
	EvidenceHash   string    `json:"evidence_hash"`
	ResolvedBy     string    `json:"resolved_by"`
	ResolvedAt     time.Time `json:"resolved_at"`
}

func NewBuildSourceResolution(execution *BuildExecution, result *BuildWorkerResult, actor string, now time.Time) (*BuildSourceResolution, error) {
	if execution == nil || result == nil {
		return nil, errors.New("build execution and worker result are required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("source resolver identity is required")
	}
	if result.Status != BuildWorkerResultComplete || result.ExecutionID != execution.ID || result.BuildPlanHash != execution.BuildPlanHash {
		return nil, errors.New("worker result does not match the queued execution")
	}
	var manifest BuildManifest
	if err := json.Unmarshal([]byte(execution.ManifestJSON), &manifest); err != nil {
		return nil, errors.New("build manifest is invalid JSON")
	}
	if manifest.SourceRepository != OfficialContribRepository {
		return nil, errors.New("source repository is not in the official allow-list")
	}
	if !officialReleaseTag.MatchString(manifest.SourceRef) {
		return nil, errors.New("official source must use a versioned release tag")
	}
	archiveURL := "https://codeload.github.com/open-telemetry/opentelemetry-collector-releases/tar.gz/refs/tags/" + manifest.SourceRef
	canonical, _ := json.Marshal(struct{ ExecutionID, WorkerResultID, PlanHash, Repository, SourceRef, ArchiveURL string }{execution.ID, result.ID, execution.BuildPlanHash, manifest.SourceRepository, manifest.SourceRef, archiveURL})
	sum := sha256.Sum256(canonical)
	evidenceHash := hex.EncodeToString(sum[:])
	return &BuildSourceResolution{ID: "build-source-" + evidenceHash[:16], ExecutionID: execution.ID, WorkerResultID: result.ID, BuildPlanHash: execution.BuildPlanHash, Repository: manifest.SourceRepository, SourceRef: manifest.SourceRef, ArchiveURL: archiveURL, Status: BuildSourceResolved, EvidenceHash: evidenceHash, ResolvedBy: actor, ResolvedAt: now.UTC()}, nil
}
