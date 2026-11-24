package storage

import (
	"context"
	"github.com/marellasunil/OTelPlane/internal/distributions"
)

type DistributionPreviewStore interface {
	Create(context.Context, *distributions.DeploymentPreview) error
	Get(context.Context, string) (*distributions.DeploymentPreview, error)
	List(context.Context, int) ([]*distributions.DeploymentPreview, error)
}
