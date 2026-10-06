package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentLifecycleApprovalStore struct { mu sync.RWMutex; items map[string]*lifecycle.Approval }
func NewComponentLifecycleApprovalStore() *ComponentLifecycleApprovalStore { return &ComponentLifecycleApprovalStore{items: map[string]*lifecycle.Approval{}} }
func (s *ComponentLifecycleApprovalStore) Create(_ context.Context, value *lifecycle.Approval) error { s.mu.Lock(); defer s.mu.Unlock(); for _, item := range s.items { if item.ValidationID == value.ValidationID { return storage.ErrComponentLifecycleApprovalConflict } }; s.items[value.ID] = lifecycle.CloneApproval(value); return nil }
func (s *ComponentLifecycleApprovalStore) Get(_ context.Context, id string) (*lifecycle.Approval, error) { s.mu.RLock(); defer s.mu.RUnlock(); value, ok := s.items[id]; if !ok { return nil, storage.ErrComponentLifecycleApprovalNotFound }; return lifecycle.CloneApproval(value), nil }
func (s *ComponentLifecycleApprovalStore) List(_ context.Context, limit int) ([]*lifecycle.Approval, error) { s.mu.RLock(); defer s.mu.RUnlock(); result := []*lifecycle.Approval{}; for _, value := range s.items { result = append(result, lifecycle.CloneApproval(value)) }; sort.Slice(result, func(i,j int) bool{return result[i].CreatedAt.After(result[j].CreatedAt)}); if limit > 0 && len(result)>limit {result=result[:limit]}; return result,nil }
func (s *ComponentLifecycleApprovalStore) Review(_ context.Context,id string,from,to lifecycle.ApprovalStatus,reviewer,comment string) error { s.mu.Lock(); defer s.mu.Unlock(); value,ok:=s.items[id]; if !ok{return storage.ErrComponentLifecycleApprovalNotFound}; if value.Status!=from{return storage.ErrComponentLifecycleApprovalConflict}; now:=time.Now().UTC(); value.Status=to; value.ReviewedBy=reviewer; value.ReviewComment=comment; value.ReviewedAt=&now; return nil }
var _ storage.ComponentLifecycleApprovalStore = (*ComponentLifecycleApprovalStore)(nil)
