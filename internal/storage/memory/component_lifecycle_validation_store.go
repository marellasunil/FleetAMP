package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentLifecycleValidationStore struct { mu sync.RWMutex; items map[string]*lifecycle.Validation }
func NewComponentLifecycleValidationStore() *ComponentLifecycleValidationStore { return &ComponentLifecycleValidationStore{items: map[string]*lifecycle.Validation{}} }
func (s *ComponentLifecycleValidationStore) Create(_ context.Context, value *lifecycle.Validation) error { s.mu.Lock(); defer s.mu.Unlock(); s.items[value.ID] = lifecycle.CloneValidation(value); return nil }
func (s *ComponentLifecycleValidationStore) Get(_ context.Context, id string) (*lifecycle.Validation, error) { s.mu.RLock(); defer s.mu.RUnlock(); value, ok := s.items[id]; if !ok { return nil, storage.ErrComponentLifecycleValidationNotFound }; return lifecycle.CloneValidation(value), nil }
func (s *ComponentLifecycleValidationStore) ListByRequest(_ context.Context, requestID string) ([]*lifecycle.Validation, error) { s.mu.RLock(); defer s.mu.RUnlock(); result := []*lifecycle.Validation{}; for _, value := range s.items { if value.RequestID == requestID { result = append(result, lifecycle.CloneValidation(value)) } }; sort.Slice(result, func(i,j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) }); return result, nil }
var _ storage.ComponentLifecycleValidationStore = (*ComponentLifecycleValidationStore)(nil)
