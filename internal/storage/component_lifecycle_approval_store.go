package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

var (
	ErrComponentLifecycleApprovalNotFound = errors.New("component lifecycle approval not found")
	ErrComponentLifecycleApprovalConflict = errors.New("component lifecycle approval state changed")
)

type ComponentLifecycleApprovalStore interface {
	Create(context.Context, *lifecycle.Approval) error
	Get(context.Context, string) (*lifecycle.Approval, error)
	List(context.Context, int) ([]*lifecycle.Approval, error)
	Review(context.Context, string, lifecycle.ApprovalStatus, lifecycle.ApprovalStatus, string, string) error
}
