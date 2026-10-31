package storage

import (
	"context"
	"errors"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

var ErrConfigurationTemplateNotFound = errors.New("configuration template not found")

type ConfigurationTemplateStore interface {
	Create(context.Context, *configs.Template) error
	Get(context.Context, string) (*configs.Template, error)
	List(context.Context) ([]*configs.Template, error)
}
