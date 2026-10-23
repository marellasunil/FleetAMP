package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionPreviewHandoffStore interface {
	Create(context.Context, *distributions.PreviewHandoff) error
	// CreateApprovalHandoff atomically persists the immutable configuration,
	// pending approval request and evidence receipt. Implementations must leave
	// none of the three records behind when any write fails.
	CreateApprovalHandoff(context.Context, *configs.Configuration, *configs.GroupDeploymentRequest, *distributions.PreviewHandoff) error
	GetByPreview(context.Context, string) (*distributions.PreviewHandoff, error)
	List(context.Context, int) ([]*distributions.PreviewHandoff, error)
}
