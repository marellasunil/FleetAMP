[Reading 12 lines from start (total: 12 lines, 0 remaining)]

package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

type GroupDeploymentRequestStore interface {
	Create(context.Context, *configs.GroupDeploymentRequest) error
	ListByGroup(context.Context, string, int) ([]*configs.GroupDeploymentRequest, error)
}