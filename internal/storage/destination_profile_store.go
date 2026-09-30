package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
)

var (
	ErrDestinationProfileNotFound = errors.New("destination profile not found")
	ErrBlueprintPatternNotFound = errors.New("blueprint pattern not found")
	ErrBlueprintBlockNotFound = errors.New("blueprint block not found")
)

type DestinationProfileStore interface {
	Create(context.Context, *blueprints.DestinationProfile) error
	Get(context.Context, string) (*blueprints.DestinationProfile, error)
	List(context.Context) ([]*blueprints.DestinationProfile, error)
	Delete(context.Context, string) error
	CreatePattern(context.Context, *blueprints.Pattern) error
	GetPattern(context.Context, string) (*blueprints.Pattern, error)
	ListPatterns(context.Context) ([]*blueprints.Pattern, error)
	DeletePattern(context.Context, string) error
	CreateBlock(context.Context, *blueprints.Block) error
	GetBlock(context.Context, string) (*blueprints.Block, error)
	ListBlocks(context.Context) ([]*blueprints.Block, error)
	DeleteBlock(context.Context, string) error
}
