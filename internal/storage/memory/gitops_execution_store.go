package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GitOpsExecutionStore struct {
	mu       sync.RWMutex
	requests map[string]*lifecycle.GitOpsExecutionRequest
	events   map[string][]*lifecycle.GitOpsExecutionEvent
}

func NewGitOpsExecutionStore() *GitOpsExecutionStore {
	return &GitOpsExecutionStore{requests: map[string]*lifecycle.GitOpsExecutionRequest{}, events: map[string][]*lifecycle.GitOpsExecutionEvent{}}
}

func (s *GitOpsExecutionStore) Create(_ context.Context, request *lifecycle.GitOpsExecutionRequest, event *lifecycle.GitOpsExecutionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.requests {
		if existing.ApprovalID == request.ApprovalID {
			return storage.ErrGitOpsExecutionConflict
		}
	}
	s.requests[request.ID] = lifecycle.CloneGitOpsExecutionRequest(request)
	s.events[request.ID] = []*lifecycle.GitOpsExecutionEvent{lifecycle.CloneGitOpsExecutionEvent(event)}
	return nil
}
func (s *GitOpsExecutionStore) Get(_ context.Context, id string) (*lifecycle.GitOpsExecutionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.requests[id]
	if !ok {
		return nil, storage.ErrGitOpsExecutionNotFound
	}
	return lifecycle.CloneGitOpsExecutionRequest(v), nil
}
func (s *GitOpsExecutionStore) List(_ context.Context, limit int) ([]*lifecycle.GitOpsExecutionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*lifecycle.GitOpsExecutionRequest, 0, len(s.requests))
	for _, v := range s.requests {
		result = append(result, lifecycle.CloneGitOpsExecutionRequest(v))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
func (s *GitOpsExecutionStore) AppendEvent(_ context.Context, event *lifecycle.GitOpsExecutionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.requests[event.ExecutionID]; !ok {
		return storage.ErrGitOpsExecutionNotFound
	}
	s.events[event.ExecutionID] = append(s.events[event.ExecutionID], lifecycle.CloneGitOpsExecutionEvent(event))
	return nil
}
func (s *GitOpsExecutionStore) ListEvents(_ context.Context, id string) ([]*lifecycle.GitOpsExecutionEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.requests[id]; !ok {
		return nil, storage.ErrGitOpsExecutionNotFound
	}
	values := s.events[id]
	result := make([]*lifecycle.GitOpsExecutionEvent, 0, len(values))
	for _, v := range values {
		result = append(result, lifecycle.CloneGitOpsExecutionEvent(v))
	}
	return result, nil
}

var _ storage.GitOpsExecutionStore = (*GitOpsExecutionStore)(nil)
