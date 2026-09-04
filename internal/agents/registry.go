// Legacy in-memory Collector registry retained for simple agent inventory helpers.
//
// Purpose:
//
//	Provides concurrency-safe upsert/list operations around the ManagedAgent
//	compatibility alias. Runtime persistence is handled through storage stores.
package agents

import "sync"

type Registry struct {
	mu         sync.RWMutex
	collectors map[string]*Collector
}

// NewRegistry creates a concurrency-safe in-memory registry for basic Collector records.
func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]*Collector),
	}
}

// Upsert inserts or replaces a Collector by instance UID.
func (r *Registry) Upsert(c *Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.collectors[c.InstanceUID] = c
}

// List returns the current Collectors as a new slice.
func (r *Registry) List() []*Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Collector, 0, len(r.collectors))

	for _, collector := range r.collectors {
		result = append(result, collector)
	}

	return result
}
