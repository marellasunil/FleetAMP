package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionReleaseStore struct {
	mu    sync.RWMutex
	items map[string]*distributions.DistributionRelease
}

func NewDistributionReleaseStore() *DistributionReleaseStore {
	return &DistributionReleaseStore{items: map[string]*distributions.DistributionRelease{}}
}
func (s *DistributionReleaseStore) Create(_ context.Context, item *distributions.DistributionRelease) error {
	if item == nil {
		return fmt.Errorf("distribution release is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[item.ID]; ok {
		return fmt.Errorf("distribution release already exists")
	}
	for _, existing := range s.items {
		if existing.DistributionID == item.DistributionID {
			return fmt.Errorf("distribution is already released")
		}
	}
	copy := *item
	s.items[item.ID] = &copy
	return nil
}
func (s *DistributionReleaseStore) List(_ context.Context, limit int) ([]*distributions.DistributionRelease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.DistributionRelease, 0, len(s.items))
	for _, item := range s.items {
		if len(out) >= limit {
			break
		}
		copy := *item
		out = append(out, &copy)
	}
	return out, nil
}
