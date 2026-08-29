// FleetAMP dynamic group domain model.
//
// Groups target managed agents through exact-match targeting metadata.
// Reported agent attributes participate in targeting, while FleetAMP-owned
// labels override reported values with the same key. Membership is dynamic.
package groups

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

type Group struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Selector    map[string]string `json:"selector"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func New(name, description string, selector map[string]string) (*Group, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Group{ID: id, Name: name, Description: description, Selector: cloneMap(selector), Enabled: true, CreatedAt: now, UpdatedAt: now}, nil
}

func Matches(group *Group, agent *agents.ManagedAgent) bool {
	if group == nil || !group.Enabled {
		return false
	}
	return MatchesIdentity(group, agent)
}

// MatchesIdentity checks assignment identity even when a group is disabled.
// It is used for safety checks such as preventing deletion of assigned groups.
func MatchesIdentity(group *Group, agent *agents.ManagedAgent) bool {
	if group == nil || agent == nil || len(group.Selector) == 0 {
		return false
	}
	metadata := GroupIdentity(agent)
	for key, expected := range group.Selector {
		if metadata[key] != expected {
			return false
		}
	}
	return true
}

// GroupIdentity returns the controlled group identity used for membership.
// Reported values provide the base and FleetAMP-managed values override them.
func GroupIdentity(agent *agents.ManagedAgent) map[string]string {
	out := map[string]string{}
	if agent == nil {
		return out
	}
	for k, v := range agent.ReportedGroupFields {
		out[k] = v
	}
	for k, v := range agent.GroupFields {
		out[k] = v
	}
	return out
}

// EffectiveLabels returns optional labels, with FleetAMP-managed values overriding
// labels reported by the Collector. Labels do not define group identity.
func EffectiveLabels(agent *agents.ManagedAgent) map[string]string {
	out := map[string]string{}
	if agent == nil {
		return out
	}
	for k, v := range agent.ReportedLabels {
		out[k] = v
	}
	for k, v := range agent.Labels {
		out[k] = v
	}
	return out
}

// TargetingMetadata is retained for compatibility with the current UI.
func TargetingMetadata(agent *agents.ManagedAgent) map[string]string {
	out := GroupIdentity(agent)
	for k, v := range EffectiveLabels(agent) {
		out[k] = v
	}
	return out
}

func newID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate group id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
