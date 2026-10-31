package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ConfigurationTemplateStore struct {
	mu    sync.RWMutex
	items map[string]*configs.Template
}

func NewConfigurationTemplateStore() *ConfigurationTemplateStore {
	return &ConfigurationTemplateStore{items: map[string]*configs.Template{}}
}

func (s *ConfigurationTemplateStore) Create(ctx context.Context, item *configs.Template) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("configuration template is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[item.ID]; exists {
		return fmt.Errorf("configuration template version already exists")
	}
	s.items[item.ID] = configs.CloneTemplate(item)
	return nil
}

func (s *ConfigurationTemplateStore) Get(ctx context.Context, id string) (*configs.Template, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[id]
	if !ok {
		return nil, storage.ErrConfigurationTemplateNotFound
	}
	return configs.CloneTemplate(item), nil
}

func (s *ConfigurationTemplateStore) List(ctx context.Context) ([]*configs.Template, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*configs.Template, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, configs.CloneTemplate(item))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

var _ storage.ConfigurationTemplateStore = (*ConfigurationTemplateStore)(nil)
