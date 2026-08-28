// Storage contract for per-agent configuration assignments.
//
// Purpose:
//   Tracks desired configuration and asynchronous delivery status independently
//   from the OpAMP adapter and from the chosen persistence implementation.

package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

var ErrAssignmentNotFound = errors.New("configuration assignment not found")

type AssignmentStore interface {
	Upsert(ctx context.Context, assignment *configs.Assignment) error
	Get(ctx context.Context, agentUID, configID string) (*configs.Assignment, error)
	List(ctx context.Context) ([]*configs.Assignment, error)
	UpdateByAgentHash(ctx context.Context, agentUID, configHash string, status configs.DeliveryStatus, errText string) error
}
