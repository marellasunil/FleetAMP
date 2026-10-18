package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildWorkerResultStore interface {
	Create(context.Context, *distributions.BuildWorkerResult) error
	List(context.Context, int) ([]*distributions.BuildWorkerResult, error)
}
