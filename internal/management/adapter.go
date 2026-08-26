package management

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

// EventType describes normalized lifecycle events produced by a management
// protocol adapter.
type EventType string

const (
	EventConnected    EventType = "connected"
	EventDisconnected EventType = "disconnected"
	EventUpdated      EventType = "updated"
)

// Event is the protocol-independent event FleetAMP consumes from management
// adapters such as OpAMP.
type Event struct {
	Type  EventType            `json:"type"`
	Agent *agents.ManagedAgent `json:"agent"`
}

// Adapter is the boundary between FleetAMP core logic and a concrete agent
// management protocol or implementation.
type Adapter interface {
	Name() string
	Start(ctx context.Context) error
	Events() <-chan Event
}
