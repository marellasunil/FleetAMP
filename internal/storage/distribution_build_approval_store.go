package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildApprovalStore interface {
	Create(context.Context, *distributions.BuildApproval) error
	List(context.Context, int) ([]*distributions.BuildApproval, error)
}
