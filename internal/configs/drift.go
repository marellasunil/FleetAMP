// Desired/effective configuration drift evaluation.
//
// Purpose:
//
//	Compares a FleetAMP desired configuration fragment with the effective
//	configuration reported by a managed agent through OpAMP and identifies
//	exact YAML paths that are missing or have different values.
//
// Comparison model:
//
//	OpAMP Supervisors can merge remote configuration with local/base config.
//	Extra effective settings are therefore allowed; FleetAMP only requires the
//	desired subtree to exist with matching values.
package configs

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

type DriftStatus string

const (
	DriftUnknown  DriftStatus = "unknown"
	DriftInSync   DriftStatus = "in_sync"
	DriftDetected DriftStatus = "drift"
)

type DriftDifference struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Desired   any    `json:"desired,omitempty"`
	Effective any    `json:"effective,omitempty"`
}

type DriftResult struct {
	Status      DriftStatus       `json:"status"`
	InSync      bool              `json:"in_sync"`
	Reason      string            `json:"reason,omitempty"`
	Differences []DriftDifference `json:"differences,omitempty"`
}

// CompareDesiredEffective checks whether desired YAML is represented in the
// effective configuration and returns path-level diagnostics for mismatches.
func CompareDesiredEffective(desired, effective string) DriftResult {
	if desired == "" {
		return DriftResult{Status: DriftUnknown, Reason: "no desired configuration"}
	}
	if effective == "" {
		return DriftResult{Status: DriftUnknown, Reason: "effective configuration has not been reported"}
	}
	var want, got any
	if err := yaml.Unmarshal([]byte(desired), &want); err != nil {
		return DriftResult{Status: DriftUnknown, Reason: fmt.Sprintf("desired configuration is not valid YAML: %v", err)}
	}
	if err := yaml.Unmarshal([]byte(effective), &got); err != nil {
		return DriftResult{Status: DriftUnknown, Reason: fmt.Sprintf("effective configuration is not valid YAML: %v", err)}
	}

	differences := make([]DriftDifference, 0)
	collectDifferences("", want, got, &differences)
	sort.Slice(differences, func(i, j int) bool { return differences[i].Path < differences[j].Path })
	if len(differences) == 0 {
		return DriftResult{Status: DriftInSync, InSync: true}
	}
	return DriftResult{
		Status:      DriftDetected,
		Reason:      fmt.Sprintf("%d desired setting(s) differ from the reported effective configuration", len(differences)),
		Differences: differences,
	}
}

// collectDifferences recursively records structural YAML differences between desired and effective configuration values.
func collectDifferences(path string, want, got any, differences *[]DriftDifference) {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*differences = append(*differences, DriftDifference{Path: displayPath(path), Kind: "type_mismatch", Desired: want, Effective: got})
			return
		}
		keys := make([]string, 0, len(w))
		for key := range w {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			childPath := joinPath(path, key)
			actual, exists := g[key]
			if !exists {
				*differences = append(*differences, DriftDifference{Path: childPath, Kind: "missing", Desired: w[key]})
				continue
			}
			collectDifferences(childPath, w[key], actual, differences)
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			*differences = append(*differences, DriftDifference{Path: displayPath(path), Kind: "type_mismatch", Desired: want, Effective: got})
			return
		}
		if len(w) != len(g) {
			*differences = append(*differences, DriftDifference{Path: displayPath(path), Kind: "list_length", Desired: len(w), Effective: len(g)})
		}
		limit := len(w)
		if len(g) < limit {
			limit = len(g)
		}
		for i := 0; i < limit; i++ {
			collectDifferences(path+"["+strconv.Itoa(i)+"]", w[i], g[i], differences)
		}
	default:
		if !reflect.DeepEqual(want, got) {
			*differences = append(*differences, DriftDifference{Path: displayPath(path), Kind: "value_mismatch", Desired: want, Effective: got})
		}
	}
}

// joinPath builds a dotted field path for a nested drift result.
func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// displayPath returns a readable root marker when a drift difference applies to the whole document.
func displayPath(path string) string {
	if path == "" {
		return "$"
	}
	return path
}
