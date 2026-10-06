package storage

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/integrations"
)

var (
	ErrIntegrationConnectionNotFound = errors.New("integration connection not found")
	ErrIntegrationConnectionConflict = errors.New("integration connection already exists")
)

type IntegrationConnectionStore interface {
	Create(context.Context, *integrations.Connection) error
	Get(context.Context, string) (*integrations.Connection, error)
	List(context.Context, int) ([]*integrations.Connection, error)
}
