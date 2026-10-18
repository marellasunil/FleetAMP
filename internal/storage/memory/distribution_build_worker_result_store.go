package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildWorkerResultStore struct {
	mu          sync.RWMutex
	items       map[string]*distributions.BuildWorkerResult
	byExecution map[string]bool
}

func NewDistributionBuildWorkerResultStore() *DistributionBuildWorkerResultStore {
	return &DistributionBuildWorkerResultStore{items: map[string]*distributions.BuildWorkerResult{}, byExecution: map[string]bool{}}
}
func (s *DistributionBuildWorkerResultStore) Create(_ context.Context, result *distributions.BuildWorkerResult) error {
	if result == nil {
		return fmt.Errorf("build worker result is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byExecution[result.ExecutionID] {
		return fmt.Errorf("build execution already has a worker result")
	}
	copy := *result
	s.items[result.ID] = &copy
	s.byExecution[result.ExecutionID] = true
	return nil
}
func (s *DistributionBuildWorkerResultStore) List(_ context.Context, limit int) ([]*distributions.BuildWorkerResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildWorkerResult, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
