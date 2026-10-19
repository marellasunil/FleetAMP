package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildAttemptStore interface {
	Create(context.Context, *distributions.BuildAttempt) error
	List(context.Context, int) ([]*distributions.BuildAttempt, error)
}

type DistributionBuildRetryRequestStore interface {
	Create(context.Context, *distributions.BuildRetryRequest) error
	List(context.Context, int) ([]*distributions.BuildRetryRequest, error)
}
