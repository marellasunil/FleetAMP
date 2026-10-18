package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildExecutionStore struct {
	mu     sync.RWMutex
	items  map[string]*distributions.BuildExecution
	byPlan map[string]bool
}

func NewDistributionBuildExecutionStore() *DistributionBuildExecutionStore {
	return &DistributionBuildExecutionStore{items: map[string]*distributions.BuildExecution{}, byPlan: map[string]bool{}}
}

func (s *DistributionBuildExecutionStore) Create(_ context.Context, execution *distributions.BuildExecution) error {
	if execution == nil {
		return fmt.Errorf("build execution is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byPlan[execution.BuildPlanID] {
		return fmt.Errorf("build plan is already queued")
	}
	copy := *execution
	s.items[execution.ID] = &copy
	s.byPlan[execution.BuildPlanID] = true
	return nil
}

func (s *DistributionBuildExecutionStore) List(_ context.Context, limit int) ([]*distributions.BuildExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildExecution, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
