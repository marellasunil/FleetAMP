package storage

import (
	"context"
	"errors"
	"time"

	"github.com/marellasunil/OTelPlane/internal/integrations"
)

var ErrIntegrationValidationNotFound = errors.New("integration validation not found")

type IntegrationValidationStore interface {
	Create(context.Context, *integrations.ConnectionValidation) error
	Latest(context.Context, string) (*integrations.ConnectionValidation, error)
	LatestUsable(context.Context, string, time.Time) (*integrations.ConnectionValidation, error)
}
