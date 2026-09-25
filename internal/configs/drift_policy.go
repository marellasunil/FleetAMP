// Configuration drift policy controls how FleetAMP reacts when an agent's
// effective configuration differs from its approved desired configuration.
package configs

import "fmt"

type DriftPolicy string

type EffectiveConfigReport struct {
	AgentInstanceUID string
	Content          string
}

const (
	DriftPolicyReport  DriftPolicy = "report_only"
	DriftPolicyEnforce DriftPolicy = "enforce"
)

func (p DriftPolicy) Valid() bool {
	return p == DriftPolicyReport || p == DriftPolicyEnforce
}

func ParseDriftPolicy(value string) (DriftPolicy, error) {
	policy := DriftPolicy(value)
	if !policy.Valid() {
		return "", fmt.Errorf("unsupported configuration drift policy %q", value)
	}
	return policy, nil
}
