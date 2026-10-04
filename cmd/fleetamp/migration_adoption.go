package main

import (
	"sort"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

type migrationAdoptionCheck struct {
	Name     string
	Status   string
	Detail   string
	Blocking bool
}

func migrationVisibleCollectors(all []*agents.ManagedAgent, visibleGroups []*groups.Group, principalRole role) []*agents.ManagedAgent {
	result := make([]*agents.ManagedAgent, 0, len(all))
	for _, agent := range all {
		if agent == nil || agent.Type != agents.AgentTypeOTelCollector || agent.Status == agents.LifecycleRetired {
			continue
		}
		if principalRole != roleAdmin {
			visible := false
			for _, group := range visibleGroups {
				if group.Enabled && groups.MatchesIdentity(group, agent) {
					visible = true
					break
				}
			}
			if !visible {
				continue
			}
		}
		result = append(result, agent)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := strings.ToLower(result[i].Name), strings.ToLower(result[j].Name)
		if left == right {
			return result[i].InstanceUID < result[j].InstanceUID
		}
		return left < right
	})
	return result
}

func migrationCollectorByID(items []*agents.ManagedAgent, instanceUID string) *agents.ManagedAgent {
	for _, item := range items {
		if item.InstanceUID == instanceUID {
			return item
		}
	}
	return nil
}

func assessCollectorAdoption(agent *agents.ManagedAgent, group *groups.Group, effectiveConfig string) ([]migrationAdoptionCheck, bool) {
	checks := []migrationAdoptionCheck{}
	add := func(name string, passed bool, readyDetail, blockedDetail string, blocking bool) {
		status, detail := "Ready", readyDetail
		if !passed {
			status, detail = "Action required", blockedDetail
		}
		checks = append(checks, migrationAdoptionCheck{Name: name, Status: status, Detail: detail, Blocking: blocking && !passed})
	}
	if agent == nil {
		return []migrationAdoptionCheck{{Name: "Collector", Status: "Action required", Detail: "Select a Collector.", Blocking: true}}, false
	}
	stableID := strings.TrimSpace(agent.Attributes[stableAgentIDAttribute])
	add("Stable logical identity", stableID != "", stableID, "Configure the fleetamp.agent.id attribute so ownership survives a protocol UID change.", true)
	add("Effective configuration", strings.TrimSpace(effectiveConfig) != "", "Reported through OpAMP.", "Wait for the Collector to report its effective configuration.", true)
	add("Target group", group != nil && group.Enabled && groups.Matches(group, agent), "Collector matches the enabled group selector.", "Update managed labels or the group selector before adoption; FleetAMP will not silently reassign it.", true)
	add("Remote configuration", hasCapability(agent.Capabilities, "accepts_remote_config"), "Collector can receive governed versions through OpAMP.", "Enable OpAMP remote configuration before a later deployment.", true)
	add("Effective-config reporting", hasCapability(agent.Capabilities, "reports_effective_config"), "Collector can verify the applied version.", "Enable effective-config reporting for post-deployment verification and drift detection.", true)
	add("Connectivity and health", agent.Connected && agent.Healthy, "Collector is connected and healthy.", "Restore connectivity and health before deployment; configuration review may continue.", false)
	ready := true
	for _, check := range checks {
		if check.Blocking {
			ready = false
			break
		}
	}
	return checks, ready
}
