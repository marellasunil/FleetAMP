package providers

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

// EnrichmentProvider adds external metadata to a managed agent without
// coupling FleetAMP's domain model to a specific CMDB/CSDM product.
type EnrichmentProvider interface {
	Name() string
	Enrich(ctx context.Context, agent *agents.ManagedAgent) (map[string]string, error)
}
