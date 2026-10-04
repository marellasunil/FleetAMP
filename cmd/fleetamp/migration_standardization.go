package main

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var canonicalCollectorSections = []string{
	"receivers",
	"processors",
	"exporters",
	"extensions",
	"connectors",
	"service",
}

func standardizeImportedConfiguration(content string, applyBaseline bool) (string, []migrationStandardizationChange, error) {
	if _, _, err := inspectImportedConfiguration(content); err != nil {
		return "", nil, err
	}
	document, err := decodeSingleYAMLDocument(content)
	if err != nil {
		return "", nil, err
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode || len(root.Content) == 0 {
		return "", nil, fmt.Errorf("Collector YAML must be a non-empty mapping")
	}

	changes := []migrationStandardizationChange{}
	if applyBaseline {
		baselineChanges, err := applyCollectorBaseline(root)
		if err != nil {
			return "", nil, err
		}
		changes = append(changes, baselineChanges...)
	}

	canonicalizeCollectorLayout(root)
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return "", nil, fmt.Errorf("encode standardized Collector YAML: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", nil, fmt.Errorf("close standardized Collector YAML encoder: %w", err)
	}
	standardized := output.String()
	if normalizeYAMLText(content) != normalizeYAMLText(standardized) {
		changes = append([]migrationStandardizationChange{{
			Category: "Layout",
			Summary:  "Applied FleetAMP canonical section order",
			Detail:   "Known Collector sections and component names are ordered deterministically; unknown top-level sections are preserved after them.",
		}}, changes...)
	}
	return standardized, changes, nil
}

func decodeSingleYAMLDocument(content string) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(strings.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse Collector YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("parse Collector YAML: %w", err)
		}
		return nil, fmt.Errorf("Collector YAML must contain exactly one document")
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("Collector YAML must contain exactly one document")
	}
	return &document, nil
}

func applyCollectorBaseline(root *yaml.Node) ([]migrationStandardizationChange, error) {
	changes := []migrationStandardizationChange{}
	processors := mappingValue(root, "processors")
	if processors == nil {
		processors = newMappingNode()
		appendMappingPair(root, "processors", processors)
	}
	if processors.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("processors must be a YAML mapping")
	}
	if !mappingHasComponentType(processors, "memory_limiter") {
		memoryLimiter := newMappingNode()
		appendMappingPair(memoryLimiter, "check_interval", scalarNode("1s"))
		appendMappingPair(memoryLimiter, "limit_mib", integerNode("512"))
		appendMappingPair(memoryLimiter, "spike_limit_mib", integerNode("128"))
		appendMappingPair(processors, "memory_limiter", memoryLimiter)
		changes = append(changes, migrationStandardizationChange{
			Category: "Baseline",
			Summary:  "Added memory_limiter processor",
			Detail:   "Uses the FleetAMP safe defaults: 512 MiB limit, 128 MiB spike limit and a 1s check interval.",
		})
	}
	if !mappingHasComponentType(processors, "batch") {
		appendMappingPair(processors, "batch", newMappingNode())
		changes = append(changes, migrationStandardizationChange{
			Category: "Baseline",
			Summary:  "Added batch processor",
			Detail:   "The existing configuration did not define any batch processor instance.",
		})
	}

	service := mappingValue(root, "service")
	if service == nil {
		return nil, fmt.Errorf("service.pipelines is required before applying the FleetAMP baseline")
	}
	if service.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("service must be a YAML mapping")
	}
	pipelines := mappingValue(service, "pipelines")
	if pipelines == nil || pipelines.Kind != yaml.MappingNode || len(pipelines.Content) == 0 {
		return nil, fmt.Errorf("service.pipelines must be a non-empty YAML mapping before applying the FleetAMP baseline")
	}
	for index := 0; index < len(pipelines.Content); index += 2 {
		pipelineName := pipelines.Content[index].Value
		pipeline := pipelines.Content[index+1]
		if pipeline.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("service.pipelines.%s must be a YAML mapping", pipelineName)
		}
		processorList := mappingValue(pipeline, "processors")
		if processorList == nil {
			processorList = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			appendMappingPair(pipeline, "processors", processorList)
		}
		if processorList.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("service.pipelines.%s.processors must be a YAML sequence", pipelineName)
		}
		added := []string{}
		if !sequenceHasComponentType(processorList, "memory_limiter") {
			processorList.Content = append([]*yaml.Node{scalarNode("memory_limiter")}, processorList.Content...)
			added = append(added, "memory_limiter")
		}
		if !sequenceHasComponentType(processorList, "batch") {
			processorList.Content = append(processorList.Content, scalarNode("batch"))
			added = append(added, "batch")
		}
		if len(added) > 0 {
			changes = append(changes, migrationStandardizationChange{
				Category: "Pipeline",
				Summary:  "Updated " + pipelineName + " processors",
				Detail:   "Added " + strings.Join(added, " and ") + "; existing processors and their order were preserved.",
			})
		}
	}
	return changes, nil
}

func canonicalizeCollectorLayout(root *yaml.Node) {
	reorderMapping(root, canonicalCollectorSections)
	for _, section := range []string{"receivers", "processors", "exporters", "extensions", "connectors"} {
		if node := mappingValue(root, section); node != nil && node.Kind == yaml.MappingNode {
			reorderMapping(node, nil)
		}
	}
	service := mappingValue(root, "service")
	if service == nil || service.Kind != yaml.MappingNode {
		return
	}
	reorderMapping(service, []string{"extensions", "pipelines", "telemetry"})
	if pipelines := mappingValue(service, "pipelines"); pipelines != nil && pipelines.Kind == yaml.MappingNode {
		reorderMapping(pipelines, nil)
	}
}

func reorderMapping(mapping *yaml.Node, preferred []string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode || len(mapping.Content) < 4 {
		return
	}
	pairs := map[string][]*yaml.Node{}
	keys := make([]string, 0, len(mapping.Content)/2)
	for index := 0; index < len(mapping.Content); index += 2 {
		key := mapping.Content[index].Value
		pairs[key] = []*yaml.Node{mapping.Content[index], mapping.Content[index+1]}
		keys = append(keys, key)
	}
	ordered := make([]*yaml.Node, 0, len(mapping.Content))
	used := map[string]bool{}
	for _, key := range preferred {
		if pair, ok := pairs[key]; ok {
			ordered = append(ordered, pair...)
			used[key] = true
		}
	}
	remaining := make([]string, 0, len(keys))
	for _, key := range keys {
		if !used[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	for _, key := range remaining {
		ordered = append(ordered, pairs[key]...)
	}
	mapping.Content = ordered
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func appendMappingPair(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content, scalarNode(key), value)
}

func mappingHasComponentType(mapping *yaml.Node, componentType string) bool {
	for index := 0; mapping != nil && index < len(mapping.Content); index += 2 {
		if collectorComponentType(mapping.Content[index].Value) == componentType {
			return true
		}
	}
	return false
}

func sequenceHasComponentType(sequence *yaml.Node, componentType string) bool {
	for _, item := range sequence.Content {
		if item.Kind == yaml.ScalarNode && collectorComponentType(item.Value) == componentType {
			return true
		}
	}
	return false
}

func collectorComponentType(name string) string {
	return strings.SplitN(name, "/", 2)[0]
}

func newMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func integerNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: value}
}

func normalizeYAMLText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
}
