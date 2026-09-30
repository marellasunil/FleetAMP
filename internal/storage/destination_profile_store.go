package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
)

var ErrDestinationProfileNotFound = errors.New("destination profile not found")

type DestinationProfileStore interface {
	Create(context.Context, *blueprints.DestinationProfile) error
	Get(context.Context, string) (*blueprints.DestinationProfile, error)
	List(context.Context) ([]*blueprints.DestinationProfile, error)
	Delete(context.Context, string) error
}
