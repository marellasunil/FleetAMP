package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildApprovalStore struct {
	mu     sync.RWMutex
	items  map[string]*distributions.BuildApproval
	byPlan map[string]bool
}

func NewDistributionBuildApprovalStore() *DistributionBuildApprovalStore {
	return &DistributionBuildApprovalStore{items: map[string]*distributions.BuildApproval{}, byPlan: map[string]bool{}}
}
func (s *DistributionBuildApprovalStore) Create(_ context.Context, a *distributions.BuildApproval) error {
	if a == nil {
		return fmt.Errorf("build approval is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byPlan[a.BuildPlanID] {
		return fmt.Errorf("build plan already has a final decision")
	}
	copy := *a
	s.items[a.ID] = &copy
	s.byPlan[a.BuildPlanID] = true
	return nil
}
func (s *DistributionBuildApprovalStore) List(_ context.Context, limit int) ([]*distributions.BuildApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildApproval, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
