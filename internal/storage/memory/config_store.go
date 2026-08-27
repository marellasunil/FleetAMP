package memory

import (
	"context"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ConfigStore struct {
	mu      sync.RWMutex
	configs map[string]*configs.Configuration
}

func NewConfigStore() *ConfigStore {
	return &ConfigStore{configs: make(map[string]*configs.Configuration)}
}

func (s *ConfigStore) Put(ctx context.Context, config *configs.Configuration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[config.ID] = cloneConfig(config)
	return nil
}

func (s *ConfigStore) Get(ctx context.Context, id string) (*configs.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	config, ok := s.configs[id]
	if !ok {
		return nil, storage.ErrConfigurationNotFound
	}
	return cloneConfig(config), nil
}

func (s *ConfigStore) List(ctx context.Context) ([]*configs.Configuration, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*configs.Configuration, 0, len(s.configs))
	for _, config := range s.configs {
		result = append(result, cloneConfig(config))
	}
	return result, nil
}

func cloneConfig(config *configs.Configuration) *configs.Configuration {
	if config == nil {
		return nil
	}
	clone := *config
	return &clone
}

var _ storage.ConfigurationStore = (*ConfigStore)(nil)
