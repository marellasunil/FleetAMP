package storage

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/configs"
)

var (
	ErrMembershipChangeNotFound = errors.New("membership change not found")
	ErrMembershipChangeConflict = errors.New("membership change state changed")
)

type MembershipChangeStore interface {
	Create(context.Context, *configs.MembershipChange) error
	Get(context.Context, string) (*configs.MembershipChange, error)
	List(context.Context, int) ([]*configs.MembershipChange, error)
	Update(context.Context, string, configs.MembershipStatus, configs.MembershipStatus, string, string, string) error
	SaveExecutionResults(context.Context, string, []configs.MembershipExecutionResult) error
}
