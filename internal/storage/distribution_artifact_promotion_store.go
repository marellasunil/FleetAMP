package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionArtifactPromotionStore interface {
	Create(context.Context, *distributions.ArtifactPromotion) error
	List(context.Context, int) ([]*distributions.ArtifactPromotion, error)
}

type DistributionArtifactPromotionReviewStore interface {
	Create(context.Context, *distributions.ArtifactPromotionReview) error
	List(context.Context, int) ([]*distributions.ArtifactPromotionReview, error)
}

type DistributionArtifactPublicationStore interface {
	Create(context.Context, *distributions.ArtifactPublication) error
	List(context.Context, int) ([]*distributions.ArtifactPublication, error)
}
