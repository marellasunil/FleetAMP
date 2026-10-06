package memory
import("context";"sort";"sync";"github.com/marellasunil/FleetAMP/internal/lifecycle";"github.com/marellasunil/FleetAMP/internal/storage")
type ComponentLifecycleExecutionStore struct{mu sync.RWMutex;items map[string]*lifecycle.ExecutionPlan}
func NewComponentLifecycleExecutionStore()*ComponentLifecycleExecutionStore{return &ComponentLifecycleExecutionStore{items:map[string]*lifecycle.ExecutionPlan{}}}
func(s *ComponentLifecycleExecutionStore)Create(_ context.Context,value *lifecycle.ExecutionPlan)error{s.mu.Lock();defer s.mu.Unlock();for _,item:=range s.items{if item.ApprovalID==value.ApprovalID{return storage.ErrComponentLifecycleExecutionConflict}};s.items[value.ID]=lifecycle.CloneExecutionPlan(value);return nil}
func(s *ComponentLifecycleExecutionStore)Get(_ context.Context,id string)(*lifecycle.ExecutionPlan,error){s.mu.RLock();defer s.mu.RUnlock();value,ok:=s.items[id];if !ok{return nil,storage.ErrComponentLifecycleExecutionNotFound};return lifecycle.CloneExecutionPlan(value),nil}
func(s *ComponentLifecycleExecutionStore)List(_ context.Context,limit int)([]*lifecycle.ExecutionPlan,error){s.mu.RLock();defer s.mu.RUnlock();result:=[]*lifecycle.ExecutionPlan{};for _,value:=range s.items{result=append(result,lifecycle.CloneExecutionPlan(value))};sort.Slice(result,func(i,j int)bool{return result[i].CreatedAt.After(result[j].CreatedAt)});if limit>0&&len(result)>limit{result=result[:limit]};return result,nil}
var _ storage.ComponentLifecycleExecutionStore=(*ComponentLifecycleExecutionStore)(nil)
