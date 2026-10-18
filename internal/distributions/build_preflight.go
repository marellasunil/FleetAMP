package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const BuildPreflightReady = "ready-for-worker"

// BuildPreflight is immutable evidence that a queued build envelope is
// internally consistent. It does not authorize or perform a build.
type BuildPreflight struct {
	ID            string    `json:"id"`
	ExecutionID   string    `json:"execution_id"`
	BuildPlanID   string    `json:"build_plan_id"`
	BuildPlanHash string    `json:"build_plan_hash"`
	ManifestHash  string    `json:"manifest_hash"`
	Status        string    `json:"status"`
	CheckedBy     string    `json:"checked_by"`
	CheckedAt     time.Time `json:"checked_at"`
}

func NewBuildPreflight(execution *BuildExecution, actor string, now time.Time) (*BuildPreflight, error) {
	if execution == nil {
		return nil, errors.New("build execution is required")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("preflight actor is required")
	}
	if execution.Status != BuildExecutionStatusQueued {
		return nil, errors.New("only queued build executions can pass preflight")
	}
	if execution.ApprovalID == "" || execution.ApprovalReviewedBy == "" {
		return nil, errors.New("approved review evidence is incomplete")
	}
	var manifest BuildManifest
	if err := json.Unmarshal([]byte(execution.ManifestJSON), &manifest); err != nil {
		return nil, errors.New("build manifest is invalid JSON")
	}
	manifestSum := sha256.Sum256([]byte(execution.ManifestJSON))
	manifestHash := hex.EncodeToString(manifestSum[:])
	if manifestHash != execution.BuildPlanHash {
		return nil, errors.New("queued manifest hash does not match the approved build plan")
	}
	if manifest.DistributionID == "" || manifest.DistributionSpecHash == "" || manifest.SourceRepository == "" || manifest.SourceRef == "" {
		return nil, errors.New("build manifest source evidence is incomplete")
	}
	if len(manifest.Outputs) == 0 {
		return nil, errors.New("build manifest has no outputs")
	}
	for _, output := range manifest.Outputs {
		if output != "oci-image" && output != "binary-archive" {
			return nil, errors.New("build manifest contains unsupported output")
		}
	}
	if !strings.HasPrefix(manifest.DestinationRef, "oci://") && !strings.HasPrefix(manifest.DestinationRef, "artifact://") {
		return nil, errors.New("build destination reference is not allowed")
	}
	if !strings.HasPrefix(manifest.CredentialRef, "secret://") {
		return nil, errors.New("build credential must remain a secret reference")
	}
	canonical, _ := json.Marshal(struct{ ExecutionID, PlanHash, ManifestHash string }{execution.ID, execution.BuildPlanHash, manifestHash})
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	return &BuildPreflight{ID: "build-preflight-" + hash[:16], ExecutionID: execution.ID, BuildPlanID: execution.BuildPlanID, BuildPlanHash: execution.BuildPlanHash, ManifestHash: manifestHash, Status: BuildPreflightReady, CheckedBy: actor, CheckedAt: now.UTC()}, nil
}
