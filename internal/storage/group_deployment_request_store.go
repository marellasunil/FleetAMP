package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

var (
	ErrGroupDeploymentRequestNotFound = errors.New("group deployment request not found")
	ErrGroupDeploymentRequestConflict = errors.New("group deployment request state changed")
)

type GroupDeploymentRequestStore interface {
	Create(context.Context, *configs.GroupDeploymentRequest) error
	Get(context.Context, string) (*configs.GroupDeploymentRequest, error)
	ListByGroup(context.Context, string, int) ([]*configs.GroupDeploymentRequest, error)
	UpdateStatus(context.Context, string, configs.GroupDeploymentRequestStatus, configs.GroupDeploymentRequestStatus) error
}
