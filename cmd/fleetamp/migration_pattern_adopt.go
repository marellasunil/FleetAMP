package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"gopkg.in/yaml.v3"
)

type migrationPatternAdoptionChange struct {
	Category string
	Summary  string
	Detail   string
}

func migrationCatalogPatterns(ctx context.Context, store storage.DestinationProfileStore) ([]*blueprints.Pattern, error) {
	patterns, err := store.ListPatterns(ctx)
	if err != nil {
		return nil, err
	}
	if len(patterns) != 0 {
		return patterns, nil
	}
	for _, starter := range blueprints.CommonStarters() {
		patterns = append(patterns, starter.Pattern)
	}
	return patterns, nil
}

func enabledPatternByID(patterns []*blueprints.Pattern, id string) *blueprints.Pattern {
	for _, pattern := range patterns {
		if pattern != nil && pattern.Enabled && pattern.ID == id {
			return pattern
		}
	}
	return nil
}

// adoptCollectorPattern replaces only the configuration of the matched
// receiver instance. Other receivers and every processor, exporter, extension,
// connector, pipeline and telemetry setting remain under the imported
// configuration's ownership. The caller must present the result for explicit
// review; this function does not save or deploy it.
func adoptCollectorPattern(content string, pattern *blueprints.Pattern) (string, []migrationPatternAdoptionChange, error) {
	if pattern == nil || !pattern.Enabled {
		return "", nil, fmt.Errorf("an enabled Pattern is required")
	}
	if _, _, err := inspectImportedConfiguration(content); err != nil {
		return "", nil, err
	}
	document, err := decodeSingleYAMLDocument(content)
	if err != nil {
		return "", nil, err
	}
	root := document.Content[0]
	receivers := mappingValue(root, "receivers")
	if receivers == nil || receivers.Kind != yaml.MappingNode {
		return "", nil, fmt.Errorf("receivers must be a YAML mapping")
	}

	names := []string{}
	for index := 0; index < len(receivers.Content); index += 2 {
		if collectorComponentType(receivers.Content[index].Value) == pattern.ReceiverID {
			names = append(names, receivers.Content[index].Value)
		}
	}
	if len(names) == 0 {
		return "", nil, fmt.Errorf("Pattern receiver %q is not present in the configuration", pattern.ReceiverID)
	}
	sort.Strings(names)
	targetName := names[0]

	patternConfig, err := decodePatternReceiverConfig(pattern.ReceiverConfig)
	if err != nil {
		return "", nil, fmt.Errorf("Pattern %q has invalid receiver configuration: %w", pattern.Name, err)
	}
	var previous *yaml.Node
	for index := 0; index < len(receivers.Content); index += 2 {
		if receivers.Content[index].Value == targetName {
			previous = receivers.Content[index+1]
			receivers.Content[index+1] = patternConfig
			break
		}
	}
	if previous == nil {
		return "", nil, fmt.Errorf("matched receiver %q disappeared during adoption", targetName)
	}

	canonicalizeCollectorLayout(root)
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return "", nil, fmt.Errorf("encode Pattern adoption preview: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", nil, fmt.Errorf("close Pattern adoption preview encoder: %w", err)
	}

	changes := []migrationPatternAdoptionChange{}
	if !yamlNodesEqual(previous, patternConfig) {
		changes = append(changes, migrationPatternAdoptionChange{
			Category: "Pattern",
			Summary:  "Align receiver " + targetName + " with " + pattern.Name,
			Detail:   "The current Pattern receiver settings replace the matched receiver settings. Other configuration sections are preserved exactly.",
		})
	}
	if len(names) > 1 {
		changes = append(changes, migrationPatternAdoptionChange{
			Category: "Scope",
			Summary:  "Left additional " + pattern.ReceiverID + " receivers unchanged",
			Detail:   "Only " + targetName + " is adopted in this step; remaining instances are preserved to avoid an ambiguous bulk change.",
		})
	}
	return output.String(), changes, nil
}

func decodePatternReceiverConfig(content string) (*yaml.Node, error) {
	if strings.TrimSpace(content) == "" {
		return newMappingNode(), nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("receiver configuration must be a YAML mapping")
	}
	return document.Content[0], nil
}

func yamlNodesEqual(left, right *yaml.Node) bool {
	leftBytes, leftErr := yaml.Marshal(left)
	rightBytes, rightErr := yaml.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftBytes) == string(rightBytes)
}

func migrationProposalHash(content, patternID string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(patternID+"\x00"+content)))
}
