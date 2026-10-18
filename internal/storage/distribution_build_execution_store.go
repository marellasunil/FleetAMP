package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildExecutionStore interface {
	Create(context.Context, *distributions.BuildExecution) error
	List(context.Context, int) ([]*distributions.BuildExecution, error)
}
