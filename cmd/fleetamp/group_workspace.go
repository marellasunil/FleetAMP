package main

import (
	"context"
	"errors"
	"sort"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type groupDriftItem struct {
	Agent *agents.ManagedAgent
	Drift configs.DriftResult
}

type groupDriftSummary struct {
	InSync  int
	Drifted int
	Unknown int
	Items   []groupDriftItem
}

func buildGroupDrift(ctx context.Context, members []*agents.ManagedAgent, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, adapter *fleetopamp.Adapter) (groupDriftSummary, error) {
	summary := groupDriftSummary{Items: make([]groupDriftItem, 0, len(members))}
	for _, agent := range members {
		result, err := driftForGroupAgent(ctx, agent.InstanceUID, configStore, assignmentStore, adapter)
		if err != nil {
			return groupDriftSummary{}, err
		}
		switch result.Status {
		case configs.DriftInSync:
			summary.InSync++
		case configs.DriftDetected:
			summary.Drifted++
		default:
			summary.Unknown++
		}
		summary.Items = append(summary.Items, groupDriftItem{Agent: agent, Drift: result})
	}
	return summary, nil
}

func driftForGroupAgent(ctx context.Context, agentUID string, configStore storage.ConfigurationStore, assignmentStore storage.AssignmentStore, adapter *fleetopamp.Adapter) (configs.DriftResult, error) {
	latest, err := latestAssignmentForAgent(ctx, assignmentStore, agentUID)
	if errors.Is(err, storage.ErrAssignmentNotFound) {
		return configs.CompareDesiredEffective("", adapter.EffectiveConfig(agentUID)), nil
	}
	if err != nil {
		return configs.DriftResult{}, err
	}
	desired, err := configStore.Get(ctx, latest.ConfigurationID)
	if err != nil {
		return configs.DriftResult{}, err
	}
	return configs.CompareDesiredEffective(desired.Content, adapter.EffectiveConfig(agentUID)), nil
}

func groupDeploymentHistory(ctx context.Context, members []*agents.ManagedAgent, store storage.DeploymentStore, limit int) ([]*configs.Deployment, error) {
	history := make([]*configs.Deployment, 0)
	for _, agent := range members {
		items, err := store.ListByAgent(ctx, agent.InstanceUID, limit)
		if err != nil {
			return nil, err
		}
		history = append(history, items...)
	}
	sort.Slice(history, func(i, j int) bool {
		return history[i].CreatedAt.After(history[j].CreatedAt)
	})
	if len(history) > limit {
		history = history[:limit]
	}
	return history, nil
}
