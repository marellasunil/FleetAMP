package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildSourceInspectionStore interface {
	Create(context.Context, *distributions.BuildSourceInspection) error
	List(context.Context, int) ([]*distributions.BuildSourceInspection, error)
}
