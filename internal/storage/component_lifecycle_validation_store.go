package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var ErrComponentLifecycleValidationNotFound = errors.New("component lifecycle validation not found")

// ComponentLifecycleValidationStore is append-only: revalidation creates a new snapshot.
type ComponentLifecycleValidationStore interface {
	Create(context.Context, *lifecycle.Validation) error
	Get(context.Context, string) (*lifecycle.Validation, error)
	ListByRequest(context.Context, string) ([]*lifecycle.Validation, error)
}
