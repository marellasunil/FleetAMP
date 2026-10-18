package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionStore interface {
	Publish(context.Context, *distributions.Distribution) error
	List(context.Context) ([]*distributions.Distribution, error)
	Get(context.Context, string) (*distributions.Distribution, error)
}
