package agents

import "sync"

type Registry struct {
	mu         sync.RWMutex
	collectors map[string]*Collector
}

func NewRegistry() *Registry {
	return &Registry{
		collectors: make(map[string]*Collector),
	}
}

func (r *Registry) Upsert(c *Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.collectors[c.InstanceUID] = c
}

func (r *Registry) List() []*Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Collector, 0, len(r.collectors))

	for _, collector := range r.collectors {
		result = append(result, collector)
	}

	return result
}
