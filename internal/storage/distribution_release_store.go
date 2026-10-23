package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionReleaseStore interface {
	Create(context.Context, *distributions.DistributionRelease) error
	List(context.Context, int) ([]*distributions.DistributionRelease, error)
}
