// Lightweight append-only file persistence for managed-agent history.
//
// Purpose:
//   Writes lifecycle events to agent-events.jsonl and supports time-based reads
//   used by the fleet UI and /api/v1/agent-events endpoint.
//
// File format:
//   One JSON object per line (JSONL), allowing append-only writes and simple
//   recovery without introducing a database dependency at this stage.
//
// Dependencies:
//   internal/events, internal/storage, and Go filesystem/JSON packages.
//
// Scope:
//   Suitable for development/small deployments; the storage interface allows a
//   future SQLite/PostgreSQL implementation without changing callers.

package file

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/marellasunil/FleetAMP/internal/events"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type EventStore struct {
	mu   sync.Mutex
	path string
}

// NewEventStore prepares the FleetAMP data directory and uses agent-events.jsonl
// as the append-only history file.
func NewEventStore(dataDir string) (*EventStore, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	return &EventStore{path: filepath.Join(dataDir, "agent-events.jsonl")}, nil
}

// Append serializes one lifecycle event as a single JSONL record. Writes are
// mutex-protected so concurrent management events cannot interleave bytes.
func (s *EventStore) Append(ctx context.Context, event *events.AgentEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(event)
}

// ListSince scans persisted JSONL history and returns events at or after the
// supplied UTC timestamp. Corrupt records are surfaced instead of silently ignored.
func (s *EventStore) ListSince(ctx context.Context, since time.Time) ([]*events.AgentEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []*events.AgentEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result := []*events.AgentEvent{}
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			var e events.AgentEvent
			if json.Unmarshal(line, &e) == nil && (since.IsZero() || !e.Timestamp.Before(since)) {
				result = append(result, &e)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

var _ storage.AgentEventStore = (*EventStore)(nil)
