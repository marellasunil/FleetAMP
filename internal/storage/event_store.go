// Storage contract for append-only managed-agent history events.
//
// Purpose:
//   Supports recording events and querying activity since a timestamp so the UI
//   can answer which agents were active during a selected time range.

package storage

import (
	"context"
	"time"

	"github.com/marellasunil/FleetAMP/internal/events"
)

type AgentEventStore interface {
	Append(ctx context.Context, event *events.AgentEvent) error
	ListSince(ctx context.Context, since time.Time) ([]*events.AgentEvent, error)
}
