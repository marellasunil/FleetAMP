// Storage contract for current ManagedAgent state.
//
// Purpose:
//   Keeps FleetAMP core logic independent of a concrete persistence engine.
//   Implementations may be in-memory, file-backed, SQLite, or PostgreSQL.
//
// Dependency:
//   internal/agents domain model plus context/errors from the Go standard library.

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
