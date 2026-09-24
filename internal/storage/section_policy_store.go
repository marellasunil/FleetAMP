package storage

import (
	"context"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

// SectionPolicyStore persists Admin-defined Operator edit permissions.
type SectionPolicyStore interface {
	List(ctx context.Context) ([]configs.SectionPolicy, error)
	SetOperatorEditable(ctx context.Context, sectionKey string, editable bool) error
}
