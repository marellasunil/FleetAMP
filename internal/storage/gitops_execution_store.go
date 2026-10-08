package storage

import (
	"context"
	"errors"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var (
	ErrGitOpsExecutionNotFound      = errors.New("GitOps execution request not found")
	ErrGitOpsExecutionConflict      = errors.New("GitOps execution request already exists")
	ErrGitOpsExecutionQueueEmpty    = errors.New("GitOps execution queue is empty")
	ErrGitOpsExecutionLeaseConflict = errors.New("GitOps execution lease is not owned by worker")
	ErrGitOpsExecutionNotRetryable  = errors.New("GitOps execution is not retryable")
)

type GitOpsExecutionStore interface {
	Create(context.Context, *lifecycle.GitOpsExecutionRequest, *lifecycle.GitOpsExecutionEvent) error
	Get(context.Context, string) (*lifecycle.GitOpsExecutionRequest, error)
	List(context.Context, int) ([]*lifecycle.GitOpsExecutionRequest, error)
	AppendEvent(context.Context, *lifecycle.GitOpsExecutionEvent) error
	ListEvents(context.Context, string) ([]*lifecycle.GitOpsExecutionEvent, error)
	Claim(context.Context, string, time.Time, time.Time) (*lifecycle.GitOpsExecutionRequest, error)
	Complete(context.Context, string, *lifecycle.GitOpsExecutionEvent) error
	Retry(context.Context, *lifecycle.GitOpsExecutionEvent, int) error
}
