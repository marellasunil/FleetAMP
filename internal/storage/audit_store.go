// Persistent storage contract for append-only audit events.
package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/audit"
)

type AuditStore interface {
	Append(ctx context.Context, event *audit.Event) error
	List(ctx context.Context, filter audit.Filter) ([]*audit.Event, error)
}
