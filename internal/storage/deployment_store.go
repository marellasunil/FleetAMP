// Storage contract for append-only per-agent configuration deployment history.
package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

var ErrDeploymentNotFound = errors.New("configuration deployment not found")

type DeploymentStore interface {
	Create(ctx context.Context, deployment *configs.Deployment) error
	Get(ctx context.Context, id string) (*configs.Deployment, error)
	ListByAgent(ctx context.Context, agentUID string, limit int) ([]*configs.Deployment, error)
	UpdateLatestByAgentHash(ctx context.Context, agentUID, configHash string, status configs.DeliveryStatus, errText string) error
}
