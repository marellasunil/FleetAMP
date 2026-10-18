package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildPreflightStore struct {
	mu          sync.RWMutex
	items       map[string]*distributions.BuildPreflight
	byExecution map[string]bool
}

func NewDistributionBuildPreflightStore() *DistributionBuildPreflightStore {
	return &DistributionBuildPreflightStore{items: map[string]*distributions.BuildPreflight{}, byExecution: map[string]bool{}}
}
func (s *DistributionBuildPreflightStore) Create(_ context.Context, receipt *distributions.BuildPreflight) error {
	if receipt == nil {
		return fmt.Errorf("build preflight is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byExecution[receipt.ExecutionID] {
		return fmt.Errorf("build execution already has preflight evidence")
	}
	copy := *receipt
	s.items[receipt.ID] = &copy
	s.byExecution[receipt.ExecutionID] = true
	return nil
}
func (s *DistributionBuildPreflightStore) List(_ context.Context, limit int) ([]*distributions.BuildPreflight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildPreflight, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
