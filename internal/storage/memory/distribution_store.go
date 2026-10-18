package memory

import (
	"context"
	"fmt"
	"sync"
	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionStore struct { mu sync.RWMutex; items map[string]*distributions.Distribution }
func NewDistributionStore() *DistributionStore { return &DistributionStore{items: map[string]*distributions.Distribution{}} }
func (s *DistributionStore) Publish(_ context.Context,d *distributions.Distribution) error { if d==nil{return fmt.Errorf("distribution is required")};s.mu.Lock();defer s.mu.Unlock();if _,ok:=s.items[d.ID];ok{return fmt.Errorf("distribution already exists")};copy:=*d;s.items[d.ID]=&copy;return nil }
func (s *DistributionStore) List(_ context.Context)([]*distributions.Distribution,error){s.mu.RLock();defer s.mu.RUnlock();out:=make([]*distributions.Distribution,0,len(s.items));for _,item:=range s.items{copy:=*item;out=append(out,&copy)};return out,nil}
func (s *DistributionStore) Get(_ context.Context,id string)(*distributions.Distribution,error){s.mu.RLock();defer s.mu.RUnlock();item,ok:=s.items[id];if !ok{return nil,fmt.Errorf("distribution %q not found",id)};copy:=*item;return &copy,nil}
