package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func membershipRuleFromForm(r *http.Request) (*groups.MembershipRule, error) {
	enabled := r.FormValue("dynamic_membership") == "true"
	if !enabled {
		return nil, nil
	}
	field := strings.TrimSpace(r.FormValue("membership_field"))
	if field == "attribute" {
		key := strings.TrimSpace(r.FormValue("membership_attribute"))
		if !validMetadataKey(key) {
			return nil, fmt.Errorf("enter a valid reported attribute key")
		}
		field = "attribute:" + key
	}
	rule := &groups.MembershipRule{
		Field:     field,
		MatchType: groups.MembershipMatchType(strings.TrimSpace(r.FormValue("membership_match_type"))),
		Pattern:   strings.TrimSpace(r.FormValue("membership_pattern")),
	}
	if err := groups.ValidateMembershipRule(rule); err != nil {
		return nil, err
	}
	return rule, nil
}

func groupMatchesQuery(group *groups.Group, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	parts := []string{group.Name, group.Description, strings.Join(group.Owners, " ")}
	for key, value := range group.Selector {
		parts = append(parts, key, value, key+"="+value)
	}
	if group.MembershipRule != nil {
		parts = append(parts, group.MembershipRule.Field, string(group.MembershipRule.MatchType), group.MembershipRule.Pattern)
	}
	return strings.Contains(strings.ToLower(strings.Join(parts, " ")), query)
}

func matchingEnabledGroups(agent *agents.ManagedAgent, all []*groups.Group) []*groups.Group {
	result := make([]*groups.Group, 0)
	for _, group := range all {
		if groups.Matches(group, agent) {
			result = append(result, group)
		}
	}
	return result
}

func buildUnassignedCollectorItems(ctx context.Context, agentStore *memory.AgentStore, allGroups []*groups.Group) ([]unassignedCollectorItem, error) {
	all, err := agentStore.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]unassignedCollectorItem, 0)
	for _, agent := range all {
		matches := matchingEnabledGroups(agent, allGroups)
		if len(matches) == 1 {
			continue
		}
		item := unassignedCollectorItem{Agent: agent}
		if len(matches) > 1 {
			names := make([]string, 0, len(matches))
			for _, group := range matches {
				names = append(names, group.Name)
			}
			sort.Strings(names)
			item.Conflict = "Matches multiple groups: " + strings.Join(names, ", ")
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Agent.Name) < strings.ToLower(result[j].Agent.Name)
	})
	return result, nil
}

func validateDynamicRuleConflicts(ctx context.Context, candidate *groups.Group, groupStore storage.GroupStore, agentStore *memory.AgentStore) error {
	if candidate.MembershipRule == nil {
		return nil
	}
	existing, err := groupStore.List(ctx)
	if err != nil {
		return err
	}
	for _, group := range existing {
		if group.ID == candidate.ID {
			continue
		}
		if group.Enabled && group.MembershipRule != nil &&
			group.MembershipRule.Field == candidate.MembershipRule.Field &&
			group.MembershipRule.MatchType == candidate.MembershipRule.MatchType &&
			group.MembershipRule.Pattern == candidate.MembershipRule.Pattern {
			return fmt.Errorf("dynamic membership rule duplicates group %s", group.Name)
		}
	}
	all, err := agentStore.List(ctx)
	if err != nil {
		return err
	}
	for _, agent := range all {
		if !groups.DynamicMatches(candidate, agent) {
			continue
		}
		matches := matchingEnabledGroups(agent, existing)
		for _, matched := range matches {
			if matched.ID != candidate.ID {
				return fmt.Errorf("dynamic rule overlaps %s for Collector %s", matched.Name, agent.Name)
			}
		}
	}
	return nil
}

type groupAssignmentSnapshot struct {
	agent       *agents.ManagedAgent
	groupFields map[string]string
	exclusions  []string
}

func assignCollectorsDuringGroupCreation(ctx context.Context, group *groups.Group, instanceUIDs []string, groupStore storage.GroupStore, agentStore *memory.AgentStore, dataDir string) error {
	if len(instanceUIDs) == 0 {
		return nil
	}
	allGroups, err := groupStore.List(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	snapshots := make([]groupAssignmentSnapshot, 0, len(instanceUIDs))
	for _, uid := range instanceUIDs {
		uid = strings.TrimSpace(uid)
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		agent, err := agentStore.Get(ctx, uid)
		if err != nil {
			return fmt.Errorf("selected Collector %q is unavailable", uid)
		}
		for _, existing := range allGroups {
			if existing.ID != group.ID && groups.Matches(existing, agent) {
				return fmt.Errorf("Collector %s already belongs to %s", agent.Name, existing.Name)
			}
		}
		snapshots = append(snapshots, groupAssignmentSnapshot{
			agent: agent, groupFields: copyStringMap(agent.GroupFields),
			exclusions: append([]string(nil), agent.ExcludedGroupIDs...),
		})
	}
	for index, item := range snapshots {
		item.agent.GroupFields = copyStringMap(group.Selector)
		item.agent.ExcludedGroupIDs = removeString(item.agent.ExcludedGroupIDs, group.ID)
		if err := agentStore.Upsert(ctx, item.agent); err != nil {
			rollbackGroupAssignments(ctx, snapshots[:index+1], agentStore)
			return err
		}
	}
	if err := saveAgentSnapshot(ctx, agentStore, dataDir); err != nil {
		rollbackGroupAssignments(ctx, snapshots, agentStore)
		return err
	}
	return nil
}

func rollbackGroupAssignments(ctx context.Context, values []groupAssignmentSnapshot, agentStore *memory.AgentStore) {
	for _, item := range values {
		item.agent.GroupFields = item.groupFields
		item.agent.ExcludedGroupIDs = item.exclusions
		_ = agentStore.Upsert(ctx, item.agent)
	}
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}
