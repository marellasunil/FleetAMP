package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionPreviewHandoffStore struct {
	mu             sync.RWMutex
	items          []*distributions.PreviewHandoff
	configurations []*configs.Configuration
	requests       []*configs.GroupDeploymentRequest
}

func (s *DistributionPreviewHandoffStore) CreateApprovalHandoff(_ context.Context, configuration *configs.Configuration, request *configs.GroupDeploymentRequest, handoff *distributions.PreviewHandoff) error {
	if configuration == nil || request == nil || handoff == nil {
		return fmt.Errorf("configuration, approval request and distribution preview handoff are required")
	}
	if request.ConfigurationID != configuration.ID || request.ConfigurationHash != configuration.Hash || handoff.ConfigurationID != configuration.ID || handoff.ConfigurationHash != configuration.Hash || handoff.ApprovalRequestID != request.ID || handoff.GroupID != configuration.GroupID || request.GroupID != configuration.GroupID {
		return fmt.Errorf("distribution handoff evidence does not match configuration and approval request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.ID == handoff.ID || item.PreviewID == handoff.PreviewID {
			return fmt.Errorf("distribution preview handoff already exists")
		}
	}
	for _, item := range s.requests {
		if item.ID == request.ID {
			return fmt.Errorf("approval request already exists")
		}
	}
	s.configurations = append(s.configurations, configuration)
	s.requests = append(s.requests, request)
	s.items = append(s.items, handoff)
	return nil
}

func NewDistributionPreviewHandoffStore() *DistributionPreviewHandoffStore {
	return &DistributionPreviewHandoffStore{}
}
func (s *DistributionPreviewHandoffStore) Create(_ context.Context, value *distributions.PreviewHandoff) error {
	if value == nil {
		return fmt.Errorf("distribution preview handoff is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range s.items {
		if item.ID == value.ID || item.PreviewID == value.PreviewID {
			return fmt.Errorf("distribution preview handoff already exists")
		}
	}
	s.items = append(s.items, value)
	return nil
}
func (s *DistributionPreviewHandoffStore) GetByPreview(_ context.Context, id string) (*distributions.PreviewHandoff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.PreviewID == id {
			return item, nil
		}
	}
	return nil, fmt.Errorf("distribution preview handoff not found")
}
func (s *DistributionPreviewHandoffStore) List(_ context.Context, limit int) ([]*distributions.PreviewHandoff, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.items) {
		limit = len(s.items)
	}
	out := make([]*distributions.PreviewHandoff, 0, limit)
	for i := len(s.items) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.items[i])
	}
	return out, nil
}
