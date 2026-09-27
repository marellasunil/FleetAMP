// Section-aware OpenTelemetry Collector configuration editing.
package configs

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SectionReceivers         = "receivers"
	SectionProcessors        = "processors"
	SectionExporters         = "exporters"
	SectionExtensions        = "extensions"
	SectionConnectors        = "connectors"
	SectionServicePipelines  = "service_pipelines"
	SectionServiceExtensions = "service_extensions"
	SectionTelemetry         = "telemetry"
)

// SectionDefinition describes one editor tab and its default Operator access.
type SectionDefinition struct {
	Key                     string
	Title                   string
	Description             string
	DefaultOperatorEditable bool
}

// ConfigurationSectionDefinitions is the stable display and composition order.
var ConfigurationSectionDefinitions = []SectionDefinition{
	{SectionReceivers, "Receivers", "Where telemetry enters the Collector.", true},
	{SectionProcessors, "Processors", "Transforms, enriches, filters, batches, or samples telemetry.", true},
	{SectionExporters, "Exporters", "Destinations and credentials used to send telemetry.", false},
	{SectionExtensions, "Extensions", "Collector capabilities that run outside telemetry pipelines.", true},
	{SectionConnectors, "Connectors", "Components that join pipelines as an exporter and receiver.", true},
	{SectionServicePipelines, "Service pipelines", "Signal routes that connect receivers, processors, and exporters.", false},
	{SectionServiceExtensions, "Service extensions", "Extensions enabled by the Collector service.", false},
	{SectionTelemetry, "Telemetry / self-monitoring", "Collector logs, metrics, traces, and resource settings.", true},
}

// SectionDefinitionByKey returns metadata for a supported section.
func SectionDefinitionByKey(key string) (SectionDefinition, bool) {
	for _, definition := range ConfigurationSectionDefinitions {
		if definition.Key == key {
			return definition, true
		}
	}
	return SectionDefinition{}, false
}

// SplitConfigurationSections extracts editor fragments while keeping each
// fragment valid YAML representing the value below its Collector key.
func SplitConfigurationSections(content string) (map[string]string, error) {
	root, err := parseConfigurationDocument(content)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(ConfigurationSectionDefinitions))
	for _, definition := range ConfigurationSectionDefinitions {
		value := sectionNode(root, definition.Key)
		if value == nil {
			result[definition.Key] = ""
			continue
		}
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode %s section: %w", definition.Title, err)
		}
		result[definition.Key] = string(encoded)
	}
	return result, nil
}

// ComposeConfigurationSections applies section fragments to a baseline
// document. Unknown top-level and service keys from the baseline are retained.
func ComposeConfigurationSections(baseline string, sections map[string]string) (string, error) {
	root, err := parseConfigurationDocument(baseline)
	if err != nil {
		return "", fmt.Errorf("parse baseline configuration: %w", err)
	}
	for _, definition := range ConfigurationSectionDefinitions {
		raw, supplied := sections[definition.Key]
		if !supplied {
			continue
		}
		var value *yaml.Node
		if strings.TrimSpace(raw) != "" {
			value, err = parseSectionFragment(definition.Key, raw)
			if err != nil {
				return "", fmt.Errorf("%s: %w", definition.Title, err)
			}
		}
		if err := setSectionNode(root, definition.Key, value); err != nil {
			return "", err
		}
	}
	document := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	encoded, err := yaml.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("encode Collector configuration: %w", err)
	}
	return string(encoded), nil
}

// ChangedConfigurationSections reports supported sections whose semantic YAML
// differs between two complete Collector configurations.
func ChangedConfigurationSections(before, after string) ([]string, error) {
	beforeSections, err := SplitConfigurationSections(before)
	if err != nil {
		return nil, err
	}
	afterSections, err := SplitConfigurationSections(after)
	if err != nil {
		return nil, err
	}
	var changed []string
	for _, definition := range ConfigurationSectionDefinitions {
		if strings.TrimSpace(beforeSections[definition.Key]) != strings.TrimSpace(afterSections[definition.Key]) {
			changed = append(changed, definition.Key)
		}
	}
	return changed, nil
}

func parseConfigurationDocument(content string) (*yaml.Node, error) {
	if strings.TrimSpace(content) == "" {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("Collector configuration must be a YAML mapping")
	}
	return document.Content[0], nil
}

func parseSectionFragment(sectionKey, content string) (*yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("section must contain one YAML value")
	}
	value := document.Content[0]
	if value.Kind != yaml.MappingNode {
		return value, nil
	}

	// The editor displays the value below each Collector key, but users often
	// paste a complete, labelled section. Accept the matching wrapper and remove
	// it before composition so `receivers:` never becomes
	// `receivers.receivers`. Service subsections may be pasted either as
	// `pipelines:` or as `service: { pipelines: ... }`.
	if parent, nested, nestedSection := sectionLocation(sectionKey); nestedSection {
		if wrapped := soleMappingValue(value, parent); wrapped != nil {
			if wrapped.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("%s must be a YAML mapping", parent)
			}
			nestedValue := mappingValue(wrapped, nested)
			if nestedValue == nil {
				return nil, fmt.Errorf("%s wrapper must contain %s", parent, nested)
			}
			return nestedValue, nil
		}
		if wrapped := soleMappingValue(value, nested); wrapped != nil {
			return wrapped, nil
		}
	} else if wrapped := soleMappingValue(value, sectionKey); wrapped != nil {
		return wrapped, nil
	}

	// A known Collector root key in a different tab is almost certainly a
	// misplaced complete section. Failing validation here is safer than saving
	// a syntactically valid but structurally incorrect desired configuration.
	if misplaced := misplacedCollectorRoot(value, sectionKey); misplaced != "" {
		return nil, fmt.Errorf("%s belongs in the %s section, not here", misplaced, sectionTitleForRoot(misplaced))
	}
	return value, nil
}

func soleMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode || len(mapping.Content) != 2 {
		return nil
	}
	if mapping.Content[0].Value != key {
		return nil
	}
	return mapping.Content[1]
}

func misplacedCollectorRoot(value *yaml.Node, sectionKey string) string {
	if value == nil || value.Kind != yaml.MappingNode || len(value.Content) != 2 {
		return ""
	}
	key := value.Content[0].Value
	for _, root := range []string{"receivers", "processors", "exporters", "extensions", "connectors", "service"} {
		if key == root && key != sectionKey {
			return key
		}
	}
	return ""
}

func sectionTitleForRoot(root string) string {
	switch root {
	case "service":
		return "Service pipelines, Service extensions, or Telemetry"
	default:
		if definition, ok := SectionDefinitionByKey(root); ok {
			return definition.Title
		}
		return root
	}
}

func sectionNode(root *yaml.Node, key string) *yaml.Node {
	if parent, nested, ok := sectionLocation(key); ok {
		service := mappingValue(root, parent)
		if service == nil || service.Kind != yaml.MappingNode {
			return nil
		}
		return mappingValue(service, nested)
	}
	return mappingValue(root, key)
}

func setSectionNode(root *yaml.Node, key string, value *yaml.Node) error {
	if parent, nested, ok := sectionLocation(key); ok {
		service := mappingValue(root, parent)
		if service == nil {
			if value == nil {
				return nil
			}
			service = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			setMappingValue(root, parent, service)
		}
		if service.Kind != yaml.MappingNode {
			return fmt.Errorf("service must be a YAML mapping")
		}
		setMappingValue(service, nested, value)
		if len(service.Content) == 0 {
			setMappingValue(root, parent, nil)
		}
		return nil
	}
	setMappingValue(root, key, value)
	return nil
}

func sectionLocation(key string) (string, string, bool) {
	switch key {
	case SectionServicePipelines:
		return "service", "pipelines", true
	case SectionServiceExtensions:
		return "service", "extensions", true
	case SectionTelemetry:
		return "service", "telemetry", true
	default:
		return "", "", false
	}
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		if value == nil {
			mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
		} else {
			mapping.Content[index+1] = value
		}
		return
	}
	if value != nil {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
	}
}
