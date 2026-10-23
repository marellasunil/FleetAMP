package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionPreviewStore struct {
	mu    sync.RWMutex
	items []*distributions.DeploymentPreview
}

func NewDistributionPreviewStore() *DistributionPreviewStore { return &DistributionPreviewStore{} }
func (s *DistributionPreviewStore) Create(_ context.Context, p *distributions.DeploymentPreview) error {
	if p == nil {
		return fmt.Errorf("preview is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.ID == p.ID {
			return fmt.Errorf("preview already exists")
		}
	}
	s.items = append(s.items, p)
	return nil
}
func (s *DistributionPreviewStore) Get(_ context.Context, id string) (*distributions.DeploymentPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("distribution preview not found")
}
func (s *DistributionPreviewStore) List(_ context.Context, limit int) ([]*distributions.DeploymentPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]*distributions.DeploymentPreview, 0, limit)
	for i := len(s.items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.items[i])
	}
	return out, nil
}
