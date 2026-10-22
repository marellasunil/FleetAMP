package storage

import (
	"context"
	"errors"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

var (
	ErrArtifactPublicationQueueEmpty    = errors.New("artifact publication queue is empty")
	ErrArtifactPublicationLeaseConflict = errors.New("artifact publication lease is not owned by worker")
)

type ArtifactPublicationWork struct {
	Publication *distributions.ArtifactPublication
	Preflight   *distributions.ArtifactPublicationPreflight
}

type DistributionArtifactPublicationExecutionStore interface {
	Claim(context.Context, string, time.Time, time.Time) (*ArtifactPublicationWork, error)
	Complete(context.Context, string, *distributions.ArtifactPublicationExecution) error
	List(context.Context, int) ([]*distributions.ArtifactPublicationExecution, error)
}
