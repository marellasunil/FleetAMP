package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildSourceAcquisitionStore struct {
	mu       sync.RWMutex
	items    map[string]*distributions.BuildSourceAcquisition
	bySource map[string]bool
}

func NewDistributionBuildSourceAcquisitionStore() *DistributionBuildSourceAcquisitionStore {
	return &DistributionBuildSourceAcquisitionStore{items: map[string]*distributions.BuildSourceAcquisition{}, bySource: map[string]bool{}}
}
func (s *DistributionBuildSourceAcquisitionStore) Create(_ context.Context, item *distributions.BuildSourceAcquisition) error {
	if item == nil {
		return fmt.Errorf("source acquisition is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bySource[item.SourceResolutionID] {
		return fmt.Errorf("source resolution already acquired")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.bySource[item.SourceResolutionID] = true
	return nil
}
func (s *DistributionBuildSourceAcquisitionStore) List(_ context.Context, limit int) ([]*distributions.BuildSourceAcquisition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildSourceAcquisition, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
