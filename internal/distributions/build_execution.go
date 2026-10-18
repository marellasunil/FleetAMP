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
	BuildExecutionStatusQueued = "queued"
	BuildExecutorDisabled      = "disabled"
)

// BuildExecution freezes the exact approved build evidence placed in the
// execution outbox. It intentionally contains no command or mutable status.
type BuildExecution struct {
	ID                 string    `json:"id"`
	BuildPlanID        string    `json:"build_plan_id"`
	BuildPlanHash      string    `json:"build_plan_hash"`
	ApprovalID         string    `json:"approval_id"`
	ApprovalReviewedBy string    `json:"approval_reviewed_by"`
	ApprovalReviewedAt time.Time `json:"approval_reviewed_at"`
	ManifestJSON       string    `json:"manifest_json"`
	Executor           string    `json:"executor"`
	Status             string    `json:"status"`
	QueuedBy           string    `json:"queued_by"`
	QueuedAt           time.Time `json:"queued_at"`
}

func NewBuildExecution(plan *BuildPlan, approval *BuildApproval, actor string, now time.Time) (*BuildExecution, error) {
	if plan == nil {
		return nil, errors.New("build plan is required")
	}
	if approval == nil {
		return nil, errors.New("build approval is required")
	}
	if approval.Decision != BuildDecisionApproved {
		return nil, errors.New("only approved build plans can be queued")
	}
	if approval.BuildPlanID != plan.ID || approval.BuildPlanHash != plan.PlanHash {
		return nil, errors.New("approval does not match the exact build plan evidence")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("queue actor is required")
	}
	canonical, _ := json.Marshal(struct {
		PlanID, PlanHash, ApprovalID, ManifestJSON string
	}{plan.ID, plan.PlanHash, approval.ID, plan.ManifestJSON})
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	return &BuildExecution{
		ID:                 "build-execution-" + hash[:16],
		BuildPlanID:        plan.ID,
		BuildPlanHash:      plan.PlanHash,
		ApprovalID:         approval.ID,
		ApprovalReviewedBy: approval.ReviewedBy,
		ApprovalReviewedAt: approval.ReviewedAt.UTC(),
		ManifestJSON:       plan.ManifestJSON,
		Executor:           BuildExecutorDisabled,
		Status:             BuildExecutionStatusQueued,
		QueuedBy:           actor,
		QueuedAt:           now.UTC(),
	}, nil
}
