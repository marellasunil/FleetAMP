package memory

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"sort"
	"sync"
)

type IntegrationConnectionStore struct {
	mu    sync.RWMutex
	items map[string]*integrations.Connection
}

func NewIntegrationConnectionStore() *IntegrationConnectionStore {
	return &IntegrationConnectionStore{items: map[string]*integrations.Connection{}}
}
func (s *IntegrationConnectionStore) Create(_ context.Context, v *integrations.Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.items {
		if x.Name == v.Name {
			return storage.ErrIntegrationConnectionConflict
		}
	}
	s.items[v.ID] = integrations.CloneConnection(v)
	return nil
}
func (s *IntegrationConnectionStore) Get(_ context.Context, id string) (*integrations.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[id]
	if !ok {
		return nil, storage.ErrIntegrationConnectionNotFound
	}
	return integrations.CloneConnection(v), nil
}
func (s *IntegrationConnectionStore) List(_ context.Context, n int) ([]*integrations.Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := []*integrations.Connection{}
	for _, v := range s.items {
		r = append(r, integrations.CloneConnection(v))
	}
	sort.Slice(r, func(i, j int) bool { return r[i].CreatedAt.After(r[j].CreatedAt) })
	if n > 0 && len(r) > n {
		r = r[:n]
	}
	return r, nil
}
