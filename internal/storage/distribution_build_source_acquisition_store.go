package storage

import (
	"context"
	"github.com/marellasunil/OTelPlane/internal/distributions"
)

type DistributionBuildSourceAcquisitionStore interface {
	Create(context.Context, *distributions.BuildSourceAcquisition) error
	List(context.Context, int) ([]*distributions.BuildSourceAcquisition, error)
}
