// Persistent storage contract for the global configuration drift policy.
package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

type DriftPolicyStore interface {
	Get(ctx context.Context) (configs.DriftPolicy, error)
	Set(ctx context.Context, policy configs.DriftPolicy) error
}
