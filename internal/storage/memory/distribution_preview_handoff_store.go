package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionPreviewHandoffStore struct {
	mu    sync.RWMutex
	items []*distributions.PreviewHandoff
}

func NewDistributionPreviewHandoffStore() *DistributionPreviewHandoffStore {
	return &DistributionPreviewHandoffStore{}
}
func (s *DistributionPreviewHandoffStore) Create(_ context.Context, value *distributions.PreviewHandoff) error {
	if value == nil {
		return fmt.Errorf("distribution preview handoff is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.ID == value.ID || item.PreviewID == value.PreviewID {
			return fmt.Errorf("distribution preview handoff already exists")
		}
	}
	s.items = append(s.items, value)
	return nil
}
func (s *DistributionPreviewHandoffStore) GetByPreview(_ context.Context, id string) (*distributions.PreviewHandoff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.PreviewID == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("distribution preview handoff not found")
}
func (s *DistributionPreviewHandoffStore) List(_ context.Context, limit int) ([]*distributions.PreviewHandoff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]*distributions.PreviewHandoff, 0, limit)
	for i := len(s.items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.items[i])
	}
	return out, nil
}
