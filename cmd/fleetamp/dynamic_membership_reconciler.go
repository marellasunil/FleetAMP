package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	fleetopamp "github.com/marellasunil/FleetAMP/internal/opamp"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

// reconcileDynamicGroupMembership assigns an otherwise unassigned Collector
// only when exactly one enabled dynamic rule matches. If that group has an
// already-approved configuration, the exact approved artifact is delivered
// before FleetAMP persists ownership.
func reconcileDynamicGroupMembership(
	ctx context.Context,
	agent *agents.ManagedAgent,
	groupStore storage.GroupStore,
	agentStore *memory.AgentStore,
	configStore storage.ConfigurationStore,
	requestStore storage.GroupDeploymentRequestStore,
	assignmentStore storage.AssignmentStore,
	deploymentStore storage.DeploymentStore,
	adapter *fleetopamp.Adapter,
	dataDir string,
) (*groups.Group, error) {
	if agent == nil || len(agent.GroupFields) != 0 || !agent.Connected {
		return nil, nil
	}
	allGroups, err := groupStore.List(ctx)
	if err != nil {
		return nil, err
	}
	matches := make([]*groups.Group, 0, 1)
	for _, group := range allGroups {
		if group.Enabled && group.MembershipRule != nil && groups.DynamicMatches(group, agent) {
			matches = append(matches, group)
		}
	}
	if len(matches) != 1 {
		return nil, nil
	}
	group := matches[0]
	configuration, err := latestApprovedGroupConfiguration(ctx, configStore, requestStore, group.ID)
	if err != nil {
		return nil, fmt.Errorf("find approved configuration: %w", err)
	}
	if configuration != nil {
		_, _, err = deliverConfiguration(ctx, agent.InstanceUID, configuration, configs.DeploymentActionDeploy, assignmentStore, deploymentStore, adapter)
		if err != nil && !errors.Is(err, configs.ErrConfigurationCurrent) {
			return nil, fmt.Errorf("deliver approved group configuration: %w", err)
		}
	}
	previousFields := copyStringMap(agent.GroupFields)
	previousExclusions := append([]string(nil), agent.ExcludedGroupIDs...)
	agent.GroupFields = copyStringMap(group.Selector)
	agent.ExcludedGroupIDs = removeString(agent.ExcludedGroupIDs, group.ID)
	if err := agentStore.Upsert(ctx, agent); err != nil {
		agent.GroupFields = previousFields
		agent.ExcludedGroupIDs = previousExclusions
		return nil, err
	}
	if err := saveAgentSnapshot(ctx, agentStore, dataDir); err != nil {
		agent.GroupFields = previousFields
		agent.ExcludedGroupIDs = previousExclusions
		_ = agentStore.Upsert(ctx, agent)
		return nil, err
	}
	return group, nil
}
