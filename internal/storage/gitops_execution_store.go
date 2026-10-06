package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var (
	ErrGitOpsExecutionNotFound = errors.New("GitOps execution request not found")
	ErrGitOpsExecutionConflict = errors.New("GitOps execution request already exists")
)

type GitOpsExecutionStore interface {
	Create(context.Context, *lifecycle.GitOpsExecutionRequest, *lifecycle.GitOpsExecutionEvent) error
	Get(context.Context, string) (*lifecycle.GitOpsExecutionRequest, error)
	List(context.Context, int) ([]*lifecycle.GitOpsExecutionRequest, error)
	AppendEvent(context.Context, *lifecycle.GitOpsExecutionEvent) error
	ListEvents(context.Context, string) ([]*lifecycle.GitOpsExecutionEvent, error)
}
