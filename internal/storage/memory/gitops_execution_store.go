package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GitOpsExecutionStore struct {
	mu       sync.RWMutex
	requests map[string]*lifecycle.GitOpsExecutionRequest
	events   map[string][]*lifecycle.GitOpsExecutionEvent
	leases   map[string]gitOpsLease
}

type gitOpsLease struct {
	worker string
	until  time.Time
}

func NewGitOpsExecutionStore() *GitOpsExecutionStore {
	return &GitOpsExecutionStore{requests: map[string]*lifecycle.GitOpsExecutionRequest{}, events: map[string][]*lifecycle.GitOpsExecutionEvent{}, leases: map[string]gitOpsLease{}}
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

func (s *GitOpsExecutionStore) Claim(_ context.Context, worker string, now, until time.Time) (*lifecycle.GitOpsExecutionRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, request := range s.requests {
		events := s.events[id]
		if len(events) == 0 {
			continue
		}
		latest := events[len(events)-1].Status
		lease, leased := s.leases[id]
		if latest != lifecycle.GitOpsExecutionQueued && !(latest == lifecycle.GitOpsExecutionClaimed && leased && !lease.until.After(now)) {
			continue
		}
		if leased && lease.until.After(now) {
			continue
		}
		s.leases[id] = gitOpsLease{worker: worker, until: until}
		event, _ := lifecycle.NewGitOpsExecutionEvent(id, lifecycle.GitOpsExecutionClaimed, worker, "Execution claimed by provider worker", map[string]string{"lease_until": until.UTC().Format(time.RFC3339Nano)})
		s.events[id] = append(s.events[id], event)
		return lifecycle.CloneGitOpsExecutionRequest(request), nil
	}
	return nil, storage.ErrGitOpsExecutionQueueEmpty
}

func (s *GitOpsExecutionStore) Retry(_ context.Context, event *lifecycle.GitOpsExecutionEvent, maxAttempts int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event == nil || event.Status != lifecycle.GitOpsExecutionQueued {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	events, ok := s.events[event.ExecutionID]
	if !ok {
		return storage.ErrGitOpsExecutionNotFound
	}
	if len(events) == 0 || events[len(events)-1].Status != lifecycle.GitOpsExecutionFailed {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	attempts := 0
	for _, existing := range events {
		if existing.Status == lifecycle.GitOpsExecutionQueued {
			attempts++
		}
	}
	if maxAttempts > 0 && attempts >= maxAttempts {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	s.events[event.ExecutionID] = append(events, lifecycle.CloneGitOpsExecutionEvent(event))
	return nil
}

func (s *GitOpsExecutionStore) Complete(_ context.Context, worker string, event *lifecycle.GitOpsExecutionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease, ok := s.leases[event.ExecutionID]
	if !ok || lease.worker != worker {
		return storage.ErrGitOpsExecutionLeaseConflict
	}
	s.events[event.ExecutionID] = append(s.events[event.ExecutionID], lifecycle.CloneGitOpsExecutionEvent(event))
	delete(s.leases, event.ExecutionID)
	return nil
}

var _ storage.GitOpsExecutionStore = (*GitOpsExecutionStore)(nil)
