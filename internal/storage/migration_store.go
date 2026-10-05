package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/migrations"
)

// MigrationStore atomically saves an immutable configuration artifact and its
// append-only migration provenance record.
type MigrationStore interface {
	Save(ctx context.Context, configuration *configs.Configuration, record *migrations.Record) error
	List(ctx context.Context) ([]*migrations.Record, error)
}
