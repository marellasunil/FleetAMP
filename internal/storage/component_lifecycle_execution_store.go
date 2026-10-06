package storage

import("context";"errors";"github.com/marellasunil/FleetAMP/internal/lifecycle")
var(ErrComponentLifecycleExecutionNotFound=errors.New("component lifecycle execution plan not found");ErrComponentLifecycleExecutionConflict=errors.New("component lifecycle execution plan already exists"))
type ComponentLifecycleExecutionStore interface{Create(context.Context,*lifecycle.ExecutionPlan)error;Get(context.Context,string)(*lifecycle.ExecutionPlan,error);List(context.Context,int)([]*lifecycle.ExecutionPlan,error)}
