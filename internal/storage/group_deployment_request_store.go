package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

type GroupDeploymentRequestStore interface {
	Create(context.Context, *configs.GroupDeploymentRequest) error
	ListByGroup(context.Context, string, int) ([]*configs.GroupDeploymentRequest, error)
}
