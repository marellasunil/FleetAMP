package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildArtifactStore interface {
	Create(context.Context, *distributions.BuildArtifact) error
	List(context.Context, int) ([]*distributions.BuildArtifact, error)
}
