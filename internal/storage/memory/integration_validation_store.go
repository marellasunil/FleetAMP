package memory

import (
	"context"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type IntegrationValidationStore struct {
	mu    sync.Mutex
	items []*integrations.ConnectionValidation
}

func NewIntegrationValidationStore() *IntegrationValidationStore {
	return &IntegrationValidationStore{}
}
func (s *IntegrationValidationStore) Create(_ context.Context, v *integrations.ConnectionValidation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *v
	s.items = append(s.items, &copy)
	return nil
}
func (s *IntegrationValidationStore) Latest(_ context.Context, connectionID string) (*integrations.ConnectionValidation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].ConnectionID == connectionID {
			copy := *s.items[i]
			return &copy, nil
		}
	}
	return nil, storage.ErrIntegrationValidationNotFound
}
func (s *IntegrationValidationStore) LatestUsable(ctx context.Context, connectionID string, now time.Time) (*integrations.ConnectionValidation, error) {
	v, err := s.Latest(ctx, connectionID)
	if err != nil || !v.Usable(now) {
		return nil, storage.ErrIntegrationValidationNotFound
	}
	return v, nil
}
