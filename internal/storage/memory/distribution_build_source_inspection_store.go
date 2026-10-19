package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildSourceInspectionStore struct {
	mu            sync.RWMutex
	items         map[string]*distributions.BuildSourceInspection
	byAcquisition map[string]bool
}

func NewDistributionBuildSourceInspectionStore() *DistributionBuildSourceInspectionStore {
	return &DistributionBuildSourceInspectionStore{items: map[string]*distributions.BuildSourceInspection{}, byAcquisition: map[string]bool{}}
}
func (s *DistributionBuildSourceInspectionStore) Create(_ context.Context, item *distributions.BuildSourceInspection) error {
	if item == nil {
		return fmt.Errorf("source inspection is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byAcquisition[item.AcquisitionID] {
		return fmt.Errorf("source acquisition already inspected")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.byAcquisition[item.AcquisitionID] = true
	return nil
}
func (s *DistributionBuildSourceInspectionStore) List(_ context.Context, limit int) ([]*distributions.BuildSourceInspection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildSourceInspection, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
