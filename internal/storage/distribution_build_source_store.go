package storage

import (
	"context"
	"github.com/marellasunil/OTelPlane/internal/distributions"
)

type DistributionBuildSourceStore interface {
	Create(context.Context, *distributions.BuildSourceResolution) error
	List(context.Context, int) ([]*distributions.BuildSourceResolution, error)
}
