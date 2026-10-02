package storage

import (
	"context"
	"errors"

	groupsecrets "github.com/marellasunil/FleetAMP/internal/secrets"
)

var ErrGroupSecretNotFound = errors.New("group secret not found")

type GroupSecretStore interface {
	Upsert(context.Context, *groupsecrets.GroupSecret) error
	Get(context.Context, string, string) (*groupsecrets.GroupSecret, error)
	ListByGroup(context.Context, string) ([]*groupsecrets.GroupSecret, error)
	Delete(context.Context, string, string) error
}

