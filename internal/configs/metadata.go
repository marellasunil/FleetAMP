// Managed FleetAMP metadata merge helpers for Collector configuration.
//
// Purpose:
//
//	Safely injects FleetAMP group and label keys into service.telemetry.resource
//	while preserving the rest of the desired OpenTelemetry Collector YAML.
package configs

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	groupPrefix = "fleetamp.group."
	labelPrefix = "fleetamp.label."
)

// MergeManagedMetadata injects FleetAMP-controlled group identity and labels
// into the Collector's own telemetry resource without replacing other config.
func MergeManagedMetadata(content string, groupFields, labels map[string]string) (string, error) {
	root := map[string]any{}
	if strings.TrimSpace(content) != "" {
		if err := yaml.Unmarshal([]byte(content), &root); err != nil {
			return "", fmt.Errorf("parse desired config: %w", err)
		}
	}
	service := ensureMap(root, "service")
	telemetry := ensureMap(service, "telemetry")
	resource := ensureMap(telemetry, "resource")

	for key := range resource {
		if strings.HasPrefix(key, groupPrefix) || strings.HasPrefix(key, labelPrefix) {
			delete(resource, key)
		}
	}
	for key, value := range groupFields {
		if value != "" {
			resource[groupPrefix+key] = value
		}
	}
	for key, value := range labels {
		if value != "" {
			resource[labelPrefix+key] = value
		}
	}

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("render desired config: %w", err)
	}
	return string(out), nil
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if current, ok := parent[key].(map[string]any); ok {
		return current
	}
	value := map[string]any{}
	parent[key] = value
	return value
}
