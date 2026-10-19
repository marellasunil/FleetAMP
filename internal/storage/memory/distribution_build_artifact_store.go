package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sync"
)

type DistributionBuildArtifactStore struct {
	mu           sync.RWMutex
	items        map[string]*distributions.BuildArtifact
	byInspection map[string]bool
}

func NewDistributionBuildArtifactStore() *DistributionBuildArtifactStore {
	return &DistributionBuildArtifactStore{items: map[string]*distributions.BuildArtifact{}, byInspection: map[string]bool{}}
}
func (s *DistributionBuildArtifactStore) Create(_ context.Context, item *distributions.BuildArtifact) error {
	if item == nil {
		return fmt.Errorf("build artifact is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byInspection[item.InspectionID] {
		return fmt.Errorf("source inspection already built")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.byInspection[item.InspectionID] = true
	return nil
}
func (s *DistributionBuildArtifactStore) List(_ context.Context, limit int) ([]*distributions.BuildArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildArtifact, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
