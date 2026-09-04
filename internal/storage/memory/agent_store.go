// In-memory ManagedAgentStore implementation.
//
// Purpose:
//   Provides concurrency-safe current-state storage for the FleetAMP process.
//   Values are cloned on read/write to prevent callers mutating shared state.
//
// Lifecycle behavior:
//   FirstSeen and lifecycle timestamps are preserved across partial updates.
//
// Limitation:
//   Process memory alone is not durable; cmd/fleetamp supplements it with an
//   agents.json snapshot until a database-backed store is introduced.

package memory

import (
	"context"
	"sync"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type AgentStore struct {
	mu     sync.RWMutex
	agents map[string]*agents.ManagedAgent
}

// NewAgentStore creates an empty concurrency-safe volatile agent inventory.
func NewAgentStore() *AgentStore {
	return &AgentStore{agents: make(map[string]*agents.ManagedAgent)}
}

// Upsert stores a defensive copy and preserves lifecycle timestamps from the
// previous record when an incremental update does not provide them.
func (s *AgentStore) Upsert(ctx context.Context, agent *agents.ManagedAgent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := cloneAgent(agent)
	if previous, ok := s.agents[agent.InstanceUID]; ok {
		if copy.FirstSeen.IsZero() {
			copy.FirstSeen = previous.FirstSeen
		}
	} else if copy.FirstSeen.IsZero() {
		copy.FirstSeen = copy.LastSeen
	}
	s.agents[agent.InstanceUID] = copy
	return nil
}

// Get returns a deep copy of one agent or storage.ErrAgentNotFound.
func (s *AgentStore) Get(ctx context.Context, instanceUID string) (*agents.ManagedAgent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent, ok := s.agents[instanceUID]
	if !ok {
		return nil, storage.ErrAgentNotFound
	}
	return cloneAgent(agent), nil
}

// List returns deep copies of all agents without exposing the store's internal maps.
func (s *AgentStore) List(ctx context.Context) ([]*agents.ManagedAgent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*agents.ManagedAgent, 0, len(s.agents))
	for _, agent := range s.agents {
		result = append(result, cloneAgent(agent))
	}
	return result, nil
}

// Delete removes an agent from volatile inventory.
func (s *AgentStore) Delete(ctx context.Context, instanceUID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.agents[instanceUID]; !ok {
		return storage.ErrAgentNotFound
	}
	delete(s.agents, instanceUID)
	return nil
}

// cloneAgent deep-copies mutable fields before values cross the store boundary.
func cloneAgent(agent *agents.ManagedAgent) *agents.ManagedAgent {
	if agent == nil {
		return nil
	}
	clone := *agent
	if agent.Attributes != nil {
		clone.Attributes = make(map[string]string, len(agent.Attributes))
		for k, v := range agent.Attributes {
			clone.Attributes[k] = v
		}
	}
	if agent.Labels != nil {
		clone.Labels = make(map[string]string, len(agent.Labels))

		for k, v := range agent.Labels {
			clone.Labels[k] = v
		}
	}
	if agent.Capabilities != nil {
		clone.Capabilities = append([]string(nil), agent.Capabilities...)
	}
	return &clone
}

var _ storage.ManagedAgentStore = (*AgentStore)(nil)
