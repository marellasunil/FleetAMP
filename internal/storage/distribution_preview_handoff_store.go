package storage

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionPreviewHandoffStore interface {
	Create(context.Context, *distributions.PreviewHandoff) error
	GetByPreview(context.Context, string) (*distributions.PreviewHandoff, error)
	List(context.Context, int) ([]*distributions.PreviewHandoff, error)
}
