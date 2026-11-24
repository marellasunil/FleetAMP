package storage

import (
	"context"
	"errors"
	"github.com/marellasunil/OTelPlane/internal/lifecycle"
)

var (
	ErrComponentGitOpsPreviewNotFound = errors.New("component GitOps preview not found")
	ErrComponentGitOpsPreviewConflict = errors.New("component GitOps preview already exists")
)

type ComponentGitOpsPreviewStore interface {
	Create(context.Context, *lifecycle.GitOpsPreview) error
	Get(context.Context, string) (*lifecycle.GitOpsPreview, error)
	GetByPlan(context.Context, string) (*lifecycle.GitOpsPreview, error)
	List(context.Context, int) ([]*lifecycle.GitOpsPreview, error)
}
