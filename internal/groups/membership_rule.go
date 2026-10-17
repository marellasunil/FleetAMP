package groups

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/agents"
)

type MembershipMatchType string

const (
	MembershipMatchWildcard MembershipMatchType = "wildcard"
	MembershipMatchRegex    MembershipMatchType = "regex"
)

type MembershipRule struct {
	Field     string              `json:"field"`
	MatchType MembershipMatchType `json:"match_type"`
	Pattern   string              `json:"pattern"`
}

func ValidateMembershipRule(rule *MembershipRule) error {
	if rule == nil {
		return nil
	}
	rule.Field = strings.TrimSpace(rule.Field)
	rule.Pattern = strings.TrimSpace(rule.Pattern)
	if rule.Field == "" || rule.Pattern == "" {
		return errors.New("dynamic membership field and pattern are required")
	}
	if !validMembershipField(rule.Field) {
		return fmt.Errorf("unsupported dynamic membership field %q", rule.Field)
	}
	switch rule.MatchType {
	case MembershipMatchWildcard:
		if _, err := path.Match(rule.Pattern, "validation"); err != nil {
			return fmt.Errorf("invalid wildcard pattern: %w", err)
		}
	case MembershipMatchRegex:
		if len(rule.Pattern) > 512 {
			return errors.New("regular expression must be 512 characters or fewer")
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return fmt.Errorf("invalid regular expression: %w", err)
		}
	default:
		return errors.New("dynamic membership match type must be wildcard or regex")
	}
	return nil
}

func DynamicMatches(group *Group, agent *agents.ManagedAgent) bool {
	if group == nil || agent == nil || group.MembershipRule == nil {
		return false
	}
	if len(agent.GroupFields) != 0 || contains(agent.ExcludedGroupIDs, group.ID) {
		return false
	}
	value := membershipFieldValue(agent, group.MembershipRule.Field)
	if value == "" {
		return false
	}
	switch group.MembershipRule.MatchType {
	case MembershipMatchWildcard:
		matched, _ := path.Match(group.MembershipRule.Pattern, value)
		return matched
	case MembershipMatchRegex:
		expression, err := regexp.Compile(group.MembershipRule.Pattern)
		return err == nil && expression.MatchString(value)
	default:
		return false
	}
}

func membershipFieldValue(agent *agents.ManagedAgent, field string) string {
	switch field {
	case "stable_id":
		return strings.TrimSpace(agent.Attributes["fleetamp.agent.id"])
	case "name":
		return strings.TrimSpace(agent.Name)
	case "display_name":
		return strings.TrimSpace(agent.DisplayName)
	case "host.name":
		if value := strings.TrimSpace(agent.Attributes["host.name"]); value != "" {
			return value
		}
		return strings.TrimSpace(agent.Hostname)
	default:
		if strings.HasPrefix(field, "attribute:") {
			return strings.TrimSpace(agent.Attributes[strings.TrimPrefix(field, "attribute:")])
		}
		return ""
	}
}

func validMembershipField(field string) bool {
	switch field {
	case "stable_id", "name", "display_name", "host.name":
		return true
	}
	if !strings.HasPrefix(field, "attribute:") {
		return false
	}
	key := strings.TrimPrefix(field, "attribute:")
	if key == "" || len(key) > 128 {
		return false
	}
	for _, char := range key {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' || char == '/' {
			continue
		}
		return false
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
