package memory

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"sort"
	"sync"
)

type ComponentGitOpsPreviewStore struct {
	mu    sync.RWMutex
	items map[string]*lifecycle.GitOpsPreview
}

func NewComponentGitOpsPreviewStore() *ComponentGitOpsPreviewStore {
	return &ComponentGitOpsPreviewStore{items: map[string]*lifecycle.GitOpsPreview{}}
}
func (s *ComponentGitOpsPreviewStore) Create(_ context.Context, v *lifecycle.GitOpsPreview) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.items {
		if x.PlanID == v.PlanID {
			return storage.ErrComponentGitOpsPreviewConflict
		}
	}
	s.items[v.ID] = lifecycle.CloneGitOpsPreview(v)
	return nil
}
func (s *ComponentGitOpsPreviewStore) Get(_ context.Context, id string) (*lifecycle.GitOpsPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.items[id]
	if !ok {
		return nil, storage.ErrComponentGitOpsPreviewNotFound
	}
	return lifecycle.CloneGitOpsPreview(v), nil
}
func (s *ComponentGitOpsPreviewStore) GetByPlan(_ context.Context, id string) (*lifecycle.GitOpsPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, v := range s.items {
		if v.PlanID == id {
			return lifecycle.CloneGitOpsPreview(v), nil
		}
	}
	return nil, storage.ErrComponentGitOpsPreviewNotFound
}
func (s *ComponentGitOpsPreviewStore) List(_ context.Context, n int) ([]*lifecycle.GitOpsPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := []*lifecycle.GitOpsPreview{}
	for _, v := range s.items {
		r = append(r, lifecycle.CloneGitOpsPreview(v))
	}
	sort.Slice(r, func(i, j int) bool { return r[i].CreatedAt.After(r[j].CreatedAt) })
	if n > 0 && len(r) > n {
		r = r[:n]
	}
	return r, nil
}
