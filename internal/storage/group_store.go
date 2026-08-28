package storage

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

var ErrGroupNotFound = errors.New("group not found")

type GroupStore interface {
	Create(ctx context.Context, group *groups.Group) error
	Update(ctx context.Context, group *groups.Group) error
	Get(ctx context.Context, id string) (*groups.Group, error)
	List(ctx context.Context) ([]*groups.Group, error)
	Delete(ctx context.Context, id string) error
}
