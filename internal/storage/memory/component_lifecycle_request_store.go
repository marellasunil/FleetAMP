package memory

import (
	"context"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentLifecycleRequestStore struct {
	mu    sync.RWMutex
	items map[string]*lifecycle.Request
}

func NewComponentLifecycleRequestStore() *ComponentLifecycleRequestStore {
	return &ComponentLifecycleRequestStore{items: map[string]*lifecycle.Request{}}
}

func (s *ComponentLifecycleRequestStore) Create(_ context.Context, request *lifecycle.Request) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[request.ID] = lifecycle.Clone(request)
	return nil
}

func (s *ComponentLifecycleRequestStore) Get(_ context.Context, id string) (*lifecycle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	request, ok := s.items[id]
	if !ok {
		return nil, storage.ErrComponentLifecycleRequestNotFound
	}
	return lifecycle.Clone(request), nil
}

func (s *ComponentLifecycleRequestStore) List(_ context.Context, limit int) ([]*lifecycle.Request, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	result := make([]*lifecycle.Request, 0, len(s.items))
	for _, request := range s.items {
		result = append(result, lifecycle.Clone(request))
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

var _ storage.ComponentLifecycleRequestStore = (*ComponentLifecycleRequestStore)(nil)
