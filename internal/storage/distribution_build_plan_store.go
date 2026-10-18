package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildPlanStore interface {
	Create(context.Context, *distributions.BuildPlan) error
	List(context.Context, int) ([]*distributions.BuildPlan, error)
}
