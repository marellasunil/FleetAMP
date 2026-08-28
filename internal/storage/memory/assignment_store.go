// In-memory AssignmentStore implementation.
//
// Purpose:
//   Tracks agent/configuration assignments and updates delivery status using the
//   agent UID plus configuration hash reported asynchronously by OpAMP.
//
// Limitation:
//   Assignment state currently resets with the FleetAMP process.

package memory

import (
	"context"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type AssignmentStore struct {
	mu          sync.RWMutex
	assignments map[string]*configs.Assignment
}

func NewAssignmentStore() *AssignmentStore {
	return &AssignmentStore{assignments: make(map[string]*configs.Assignment)}
}

func assignmentKey(agentUID, configID string) string {
	return agentUID + "|" + configID
}

func (s *AssignmentStore) Upsert(ctx context.Context, assignment *configs.Assignment) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *assignment
	s.assignments[assignmentKey(assignment.AgentInstanceUID, assignment.ConfigurationID)] = &clone
	return nil
}

func (s *AssignmentStore) Get(ctx context.Context, agentUID, configID string) (*configs.Assignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	assignment, ok := s.assignments[assignmentKey(agentUID, configID)]
	if !ok {
		return nil, storage.ErrAssignmentNotFound
	}
	clone := *assignment
	return &clone, nil
}

func (s *AssignmentStore) List(ctx context.Context) ([]*configs.Assignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*configs.Assignment, 0, len(s.assignments))
	for _, assignment := range s.assignments {
		clone := *assignment
		result = append(result, &clone)
	}
	return result, nil
}

func (s *AssignmentStore) UpdateByAgentHash(ctx context.Context, agentUID, configHash string, status configs.DeliveryStatus, errText string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, assignment := range s.assignments {
		if assignment.AgentInstanceUID == agentUID && assignment.ConfigurationHash == configHash {
			assignment.Status = status
			assignment.Error = errText
			assignment.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return storage.ErrAssignmentNotFound
}

var _ storage.AssignmentStore = (*AssignmentStore)(nil)
