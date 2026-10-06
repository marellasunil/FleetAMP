package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var (
	ErrGitOpsPreviewApprovalNotFound = errors.New("GitOps preview approval not found")
	ErrGitOpsPreviewApprovalConflict = errors.New("GitOps preview approval state changed")
)

type GitOpsPreviewApprovalStore interface {
	Create(context.Context, *lifecycle.GitOpsPreviewApproval) error
	Get(context.Context, string) (*lifecycle.GitOpsPreviewApproval, error)
	List(context.Context, int) ([]*lifecycle.GitOpsPreviewApproval, error)
	Review(context.Context, string, lifecycle.ApprovalStatus, lifecycle.ApprovalStatus, string, string) error
}
