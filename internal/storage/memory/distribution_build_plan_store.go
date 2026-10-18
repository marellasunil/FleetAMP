package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildPlanStore struct {
	mu    sync.RWMutex
	items map[string]*distributions.BuildPlan
}

func NewDistributionBuildPlanStore() *DistributionBuildPlanStore {
	return &DistributionBuildPlanStore{items: map[string]*distributions.BuildPlan{}}
}
func (s *DistributionBuildPlanStore) Create(_ context.Context, p *distributions.BuildPlan) error {
	if p == nil {
		return fmt.Errorf("build plan is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[p.ID]; ok {
		return fmt.Errorf("build plan already exists")
	}
	copy := *p
	s.items[p.ID] = &copy
	return nil
}
func (s *DistributionBuildPlanStore) List(_ context.Context, limit int) ([]*distributions.BuildPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildPlan, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
