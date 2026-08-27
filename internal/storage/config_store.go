package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

var ErrConfigurationNotFound = errors.New("configuration not found")

// ConfigurationStore persists immutable FleetAMP configuration artifacts.
type ConfigurationStore interface {
	Put(ctx context.Context, config *configs.Configuration) error
	Get(ctx context.Context, id string) (*configs.Configuration, error)
	List(ctx context.Context) ([]*configs.Configuration, error)
}
