package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

var ErrAgentNotFound = errors.New("managed agent not found")

// ManagedAgentStore defines persistence operations for managed agents.
// FleetAMP core code depends on this interface, not on a database engine.
type ManagedAgentStore interface {
	Upsert(ctx context.Context, agent *agents.ManagedAgent) error
	Get(ctx context.Context, instanceUID string) (*agents.ManagedAgent, error)
	List(ctx context.Context) ([]*agents.ManagedAgent, error)
	Delete(ctx context.Context, instanceUID string) error
}
