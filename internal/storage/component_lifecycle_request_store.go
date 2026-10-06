package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var ErrComponentLifecycleRequestNotFound = errors.New("component lifecycle request not found")

// ComponentLifecycleRequestStore deliberately exposes no update operation.
// Workflow transitions must be recorded separately from the immutable proposal.
type ComponentLifecycleRequestStore interface {
	Create(context.Context, *lifecycle.Request) error
	Get(context.Context, string) (*lifecycle.Request, error)
	List(context.Context, int) ([]*lifecycle.Request, error)
}
