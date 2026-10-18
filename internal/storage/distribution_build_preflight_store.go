package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildPreflightStore interface {
	Create(context.Context, *distributions.BuildPreflight) error
	List(context.Context, int) ([]*distributions.BuildPreflight, error)
}
