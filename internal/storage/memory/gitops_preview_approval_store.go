package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GitOpsPreviewApprovalStore struct {
	mu    sync.RWMutex
	items map[string]*lifecycle.GitOpsPreviewApproval
}

func NewGitOpsPreviewApprovalStore() *GitOpsPreviewApprovalStore {
	return &GitOpsPreviewApprovalStore{items: map[string]*lifecycle.GitOpsPreviewApproval{}}
}

func (s *GitOpsPreviewApprovalStore) Create(_ context.Context, value *lifecycle.GitOpsPreviewApproval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.items {
		if existing.PreviewID == value.PreviewID {
			return storage.ErrGitOpsPreviewApprovalConflict
		}
	}
	s.items[value.ID] = lifecycle.CloneGitOpsPreviewApproval(value)
	return nil
}

func (s *GitOpsPreviewApprovalStore) Get(_ context.Context, id string) (*lifecycle.GitOpsPreviewApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.items[id]
	if !ok {
		return nil, storage.ErrGitOpsPreviewApprovalNotFound
	}
	return lifecycle.CloneGitOpsPreviewApproval(value), nil
}

func (s *GitOpsPreviewApprovalStore) List(_ context.Context, limit int) ([]*lifecycle.GitOpsPreviewApproval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*lifecycle.GitOpsPreviewApproval, 0, len(s.items))
	for _, value := range s.items {
		result = append(result, lifecycle.CloneGitOpsPreviewApproval(value))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (s *GitOpsPreviewApprovalStore) Review(_ context.Context, id string, from, to lifecycle.ApprovalStatus, reviewer, comment string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.items[id]
	if !ok {
		return storage.ErrGitOpsPreviewApprovalNotFound
	}
	if value.Status != from {
		return storage.ErrGitOpsPreviewApprovalConflict
	}
	now := time.Now().UTC()
	value.Status, value.ReviewedBy, value.ReviewComment, value.ReviewedAt = to, reviewer, comment, &now
	return nil
}

var _ storage.GitOpsPreviewApprovalStore = (*GitOpsPreviewApprovalStore)(nil)
