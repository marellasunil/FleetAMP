package storage

import (
	"context"

	"github.com/marellasunil/OTelPlane/internal/distributions"
)

type DistributionBuildExecutionStore interface {
	Create(context.Context, *distributions.BuildExecution) error
	List(context.Context, int) ([]*distributions.BuildExecution, error)
	Get(context.Context, string) (*distributions.BuildExecution, error)
}
