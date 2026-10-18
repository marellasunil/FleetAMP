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
	BuildWorkerDryRun         = "dry-run"
	BuildWorkerResultComplete = "dry-run-complete"
)

// BuildWorkerResult is immutable evidence emitted by the disabled-by-default
// dry-run worker. It records no artifact because no build is performed.
type BuildWorkerResult struct {
	ID            string    `json:"id"`
	ExecutionID   string    `json:"execution_id"`
	PreflightID   string    `json:"preflight_id"`
	BuildPlanHash string    `json:"build_plan_hash"`
	ManifestHash  string    `json:"manifest_hash"`
	WorkerID      string    `json:"worker_id"`
	Adapter       string    `json:"adapter"`
	Status        string    `json:"status"`
	EvidenceHash  string    `json:"evidence_hash"`
	CompletedAt   time.Time `json:"completed_at"`
}

func NewDryRunBuildWorkerResult(execution *BuildExecution, preflight *BuildPreflight, workerID string, now time.Time) (*BuildWorkerResult, error) {
	if execution == nil || preflight == nil {
		return nil, errors.New("build execution and preflight are required")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, errors.New("worker ID is required")
	}
	if preflight.Status != BuildPreflightReady {
		return nil, errors.New("build preflight is not ready for a worker")
	}
	if preflight.ExecutionID != execution.ID || preflight.BuildPlanID != execution.BuildPlanID || preflight.BuildPlanHash != execution.BuildPlanHash {
		return nil, errors.New("preflight does not match the queued execution")
	}
	manifestSum := sha256.Sum256([]byte(execution.ManifestJSON))
	manifestHash := hex.EncodeToString(manifestSum[:])
	if manifestHash != execution.BuildPlanHash || manifestHash != preflight.ManifestHash {
		return nil, errors.New("worker evidence hash mismatch")
	}
	canonical, _ := json.Marshal(struct{ ExecutionID, PreflightID, PlanHash, ManifestHash, WorkerID, Adapter string }{execution.ID, preflight.ID, execution.BuildPlanHash, manifestHash, workerID, BuildWorkerDryRun})
	evidenceSum := sha256.Sum256(canonical)
	evidenceHash := hex.EncodeToString(evidenceSum[:])
	return &BuildWorkerResult{ID: "build-worker-" + evidenceHash[:16], ExecutionID: execution.ID, PreflightID: preflight.ID, BuildPlanHash: execution.BuildPlanHash, ManifestHash: manifestHash, WorkerID: workerID, Adapter: BuildWorkerDryRun, Status: BuildWorkerResultComplete, EvidenceHash: evidenceHash, CompletedAt: now.UTC()}, nil
}
