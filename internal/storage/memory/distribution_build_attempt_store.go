package memory

import (
	"context"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"sort"
	"sync"
)

type DistributionBuildAttemptStore struct {
	mu       sync.RWMutex
	items    map[string]*distributions.BuildAttempt
	sequence map[string]bool
}

func NewDistributionBuildAttemptStore() *DistributionBuildAttemptStore {
	return &DistributionBuildAttemptStore{items: map[string]*distributions.BuildAttempt{}, sequence: map[string]bool{}}
}
func (s *DistributionBuildAttemptStore) Create(_ context.Context, item *distributions.BuildAttempt) error {
	if item == nil {
		return fmt.Errorf("build attempt is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s/%d", item.InspectionID, item.AttemptNumber)
	if s.sequence[key] {
		return fmt.Errorf("build attempt number already exists")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.sequence[key] = true
	return nil
}
func (s *DistributionBuildAttemptStore) List(_ context.Context, limit int) ([]*distributions.BuildAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildAttempt, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CompletedAt.After(out[j].CompletedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type DistributionBuildRetryRequestStore struct {
	mu        sync.RWMutex
	items     map[string]*distributions.BuildRetryRequest
	byAttempt map[string]bool
}

func NewDistributionBuildRetryRequestStore() *DistributionBuildRetryRequestStore {
	return &DistributionBuildRetryRequestStore{items: map[string]*distributions.BuildRetryRequest{}, byAttempt: map[string]bool{}}
}
func (s *DistributionBuildRetryRequestStore) Create(_ context.Context, item *distributions.BuildRetryRequest) error {
	if item == nil {
		return fmt.Errorf("build retry request is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byAttempt[item.PreviousAttemptID] {
		return fmt.Errorf("retry already requested for this attempt")
	}
	copy := *item
	s.items[item.ID] = &copy
	s.byAttempt[item.PreviousAttemptID] = true
	return nil
}
func (s *DistributionBuildRetryRequestStore) List(_ context.Context, limit int) ([]*distributions.BuildRetryRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	out := make([]*distributions.BuildRetryRequest, 0, len(s.items))
	for _, item := range s.items {
		copy := *item
		out = append(out, &copy)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestedAt.After(out[j].RequestedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
