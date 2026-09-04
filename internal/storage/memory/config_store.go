// In-memory ConfigurationStore implementation.
//
// Purpose:
//   Stores immutable configuration artifacts for the current FleetAMP process
//   using a concurrency-safe map and defensive copies.
//
// Limitation:
//   Configuration persistence will move to a durable store in a later milestone.

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

// NewConfigStore creates an empty volatile configuration store.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{configs: make(map[string]*configs.Configuration)}
}

// Put stores an independent copy of a versioned configuration by ID.
func (s *ConfigStore) Put(ctx context.Context, config *configs.Configuration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs[config.ID] = cloneConfig(config)
	return nil
}

// Get returns a configuration copy or storage.ErrConfigurationNotFound.
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

// List returns copies of all configurations in the volatile store.
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

// cloneConfig copies a configuration before storing or returning it.
func cloneConfig(config *configs.Configuration) *configs.Configuration {
	if config == nil {
		return nil
	}
	clone := *config
	return &clone
}

var _ storage.ConfigurationStore = (*ConfigStore)(nil)
