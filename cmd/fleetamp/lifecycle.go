// Agent lifecycle, retention, snapshot persistence, and time-range filtering.
//
// Purpose:
//   Implements connected -> disconnected -> retired lifecycle behavior and
//   supports historical fleet views such as 15m, 1h, 24h, 7d, and 30d.
//
// Runtime flow:
//   management event -> current ManagedAgent state -> agents.json snapshot
//   + agent-events.jsonl history -> retirement loop / time-range queries.
//
// Main dependencies:
//   internal/agents, internal/events, internal/storage, and the in-memory
//   ManagedAgentStore used by the current single-process FleetAMP runtime.
//
// Persistence:
//   agents.json stores latest state; event history is handled by EventStore.
//   File persistence is intentionally replaceable by SQLite/PostgreSQL later.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/events"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type agentListItem struct {
	Agent          *agents.ManagedAgent
	LastDeployment *configs.Deployment
	Groups         []*groups.Group
}

type agentListView struct {
	Page           string
	Items          []agentListItem
	Range          string
	Groups         []*groups.Group
	SelectedGroup  string
	Total          int
	Healthy        int
	Attention      int
	HealthyPercent int
}

// runRetirementLoop evaluates disconnected agents once per minute and moves
// those beyond the configured grace period into the retired lifecycle state.
func runRetirementLoop(ctx context.Context, store *memory.AgentStore, eventStore storage.AgentEventStore, retireAfter time.Duration, dataDir string) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		retireDisconnected(ctx, store, eventStore, retireAfter, dataDir)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// retireDisconnected performs one retirement pass and records a Retired event
// plus a durable snapshot whenever an agent crosses the retention threshold.
func retireDisconnected(ctx context.Context, store *memory.AgentStore, eventStore storage.AgentEventStore, retireAfter time.Duration, dataDir string) {
	items, err := store.List(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, agent := range items {
		if agent.Status != agents.LifecycleDisconnected || agent.DisconnectedAt == nil {
			continue
		}
		if now.Sub(*agent.DisconnectedAt) < retireAfter {
			continue
		}
		agent.Status = agents.LifecycleRetired
		agent.RetiredAt = &now
		if store.Upsert(ctx, agent) == nil {
			_ = eventStore.Append(ctx, &events.AgentEvent{AgentInstanceUID: agent.InstanceUID, AgentName: agent.Name, Type: events.Retired, Timestamp: now})
			_ = saveAgentSnapshot(ctx, store, dataDir)
		}
	}
}

// timeRangeStart converts UI/API range keys into an absolute UTC lower bound.
// A zero time means no lower bound (for example the all-known view).
func timeRangeStart(key string) time.Time {
	now := time.Now().UTC()
	switch key {
	case "15m":
		return now.Add(-15 * time.Minute)
	case "1h":
		return now.Add(-time.Hour)
	case "24h":
		return now.Add(-24 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	default:
		return time.Time{}
	}
}

// selectAgentsForRange returns current agent records whose lifecycle activity
// intersects the selected history window. The active view excludes retired agents.
func selectAgentsForRange(ctx context.Context, store *memory.AgentStore, eventStore interface {
	ListSince(context.Context, time.Time) ([]*events.AgentEvent, error)
}, key string) ([]*agents.ManagedAgent, error) {
	items, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	if key == "active" {
		result := items[:0]
		for _, a := range items {
			if a.Status != agents.LifecycleRetired {
				result = append(result, a)
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].LastSeen.After(result[j].LastSeen) })
		return result, nil
	}
	if key == "all" {
		sort.Slice(items, func(i, j int) bool { return items[i].LastSeen.After(items[j].LastSeen) })
		return items, nil
	}
	eventsList, err := eventStore.ListSince(ctx, timeRangeStart(key))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range eventsList {
		seen[e.AgentInstanceUID] = true
	}
	result := []*agents.ManagedAgent{}
	for _, a := range items {
		if seen[a.InstanceUID] {
			result = append(result, a)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].LastSeen.After(result[j].LastSeen) })
	return result, nil
}

// loadAgentSnapshot restores latest known state after a FleetAMP restart. Any
// agent that was previously connected is conservatively restored as disconnected
// until it reconnects and proves liveness through the management protocol.
func loadAgentSnapshot(ctx context.Context, store *memory.AgentStore, dataDir string) error {
	data, err := os.ReadFile(filepath.Join(dataDir, "agents.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var items []*agents.ManagedAgent
	if err := json.Unmarshal(data, &items); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, agent := range items {
		if agent.Connected {
			agent.Connected = false
			agent.Healthy = false
			agent.Status = agents.LifecycleDisconnected
			agent.DisconnectedAt = &now
		}
		if err := store.Upsert(ctx, agent); err != nil {
			return err
		}
	}
	return nil
}

// saveAgentSnapshot atomically persists latest agent state by writing a temporary
// JSON file and renaming it over agents.json, avoiding partially-written snapshots.
func saveAgentSnapshot(ctx context.Context, store *memory.AgentStore, dataDir string) error {
	items, err := store.List(ctx)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return err
	}
	tmp := filepath.Join(dataDir, "agents.json.tmp")
	dst := filepath.Join(dataDir, "agents.json")
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
