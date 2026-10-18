package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildSourceStore struct {
	mu          sync.RWMutex
	items       map[string]*distributions.BuildSourceResolution
	byExecution map[string]bool
}

func NewDistributionBuildSourceStore() *DistributionBuildSourceStore {
	return &DistributionBuildSourceStore{items: map[string]*distributions.BuildSourceResolution{}, byExecution: map[string]bool{}}
}
func (s *DistributionBuildSourceStore) Create(_ context.Context, item *distributions.BuildSourceResolution) error {
	if item == nil {
		return fmt.Errorf("build source resolution is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byExecution[item.ExecutionID] {
		return fmt.Errorf("build execution already has source resolution")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.byExecution[item.ExecutionID] = true
	return nil
}
func (s *DistributionBuildSourceStore) List(_ context.Context, limit int) ([]*distributions.BuildSourceResolution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildSourceResolution, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
