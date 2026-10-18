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
	BuildDecisionApproved = "approved"
	BuildDecisionRejected = "rejected"
)

type BuildApproval struct {
	ID            string    `json:"id"`
	BuildPlanID   string    `json:"build_plan_id"`
	BuildPlanHash string    `json:"build_plan_hash"`
	Decision      string    `json:"decision"`
	Reason        string    `json:"reason"`
	ReviewedBy    string    `json:"reviewed_by"`
	ReviewedAt    time.Time `json:"reviewed_at"`
}

func NewBuildApproval(plan *BuildPlan, decision, reason, reviewer string, now time.Time) (*BuildApproval, error) {
	if plan == nil {
		return nil, errors.New("build plan is required")
	}
	decision = strings.TrimSpace(decision)
	reason = strings.TrimSpace(reason)
	reviewer = strings.TrimSpace(reviewer)
	if decision != BuildDecisionApproved && decision != BuildDecisionRejected {
		return nil, errors.New("decision must be approved or rejected")
	}
	if reviewer == "" {
		return nil, errors.New("reviewer is required")
	}
	if reviewer == plan.RequestedBy {
		return nil, errors.New("build-plan requester cannot approve or reject their own plan")
	}
	if reason == "" {
		return nil, errors.New("review reason is required")
	}
	canonical, _ := json.Marshal(struct{ PlanID, PlanHash, Decision, Reason, Reviewer string }{plan.ID, plan.PlanHash, decision, reason, reviewer})
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	return &BuildApproval{ID: "build-review-" + hash[:16], BuildPlanID: plan.ID, BuildPlanHash: plan.PlanHash, Decision: decision, Reason: reason, ReviewedBy: reviewer, ReviewedAt: now.UTC()}, nil
}
