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

func NewAgentStore() *AgentStore {
	return &AgentStore{agents: make(map[string]*agents.ManagedAgent)}
}

func (s *AgentStore) Upsert(ctx context.Context, agent *agents.ManagedAgent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[agent.InstanceUID] = cloneAgent(agent)
	return nil
}

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
