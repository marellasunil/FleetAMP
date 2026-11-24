// Vendor-neutral managed-agent enrichment provider contract.
//
// Purpose:
//
//	Allows external inventory/CMDB systems to add metadata without coupling
//	OTelPlane core domain objects to a specific enterprise product.
package providers

import (
	"context"

	"github.com/marellasunil/OTelPlane/internal/agents"
)

// EnrichmentProvider adds external metadata to a managed agent without
// coupling OTelPlane's domain model to a specific CMDB/CSDM product.
type EnrichmentProvider interface {
	Name() string
	Enrich(ctx context.Context, agent *agents.ManagedAgent) (map[string]string, error)
}
