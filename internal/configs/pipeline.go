package configs

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PipelineModel is a safe, normalized view of the Collector service graph.
// It contains component names and relationships only; component settings,
// endpoints, headers, and credentials are intentionally excluded.
type PipelineModel struct {
	Pipelines  []Pipeline `json:"pipelines"`
	Extensions []string   `json:"extensions,omitempty"`
	Warnings   []string   `json:"warnings,omitempty"`
}

// Pipeline describes one service pipeline in its configured execution order.
type Pipeline struct {
	Name       string   `json:"name"`
	Signal     string   `json:"signal"`
	Receivers  []string `json:"receivers"`
	Processors []string `json:"processors"`
	Exporters  []string `json:"exporters"`
}

type collectorPipelineDocument struct {
	Receivers  map[string]yaml.Node `yaml:"receivers"`
	Processors map[string]yaml.Node `yaml:"processors"`
	Exporters  map[string]yaml.Node `yaml:"exporters"`
	Extensions map[string]yaml.Node `yaml:"extensions"`
	Service    struct {
		Extensions []string                        `yaml:"extensions"`
		Pipelines  map[string]collectorRawPipeline `yaml:"pipelines"`
	} `yaml:"service"`
}

type collectorRawPipeline struct {
	Receivers  []string `yaml:"receivers"`
	Processors []string `yaml:"processors"`
	Exporters  []string `yaml:"exporters"`
}

// ParsePipelineModel parses Collector YAML into the relationship-only model
// used by FleetAMP's validation and visualization layers.
func ParsePipelineModel(content string) (*PipelineModel, error) {
	var document collectorPipelineDocument
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("parse Collector pipeline structure: %w", err)
	}

	model := &PipelineModel{}
	usedReceivers := make(map[string]bool)
	usedProcessors := make(map[string]bool)
	usedExporters := make(map[string]bool)
	usedExtensions := make(map[string]bool)

	for _, name := range sortedKeys(document.Service.Pipelines) {
		raw := document.Service.Pipelines[name]
		signal := strings.SplitN(name, "/", 2)[0]
		if !supportedSignal(signal) {
			return nil, fmt.Errorf("pipeline %q uses unsupported signal %q", name, signal)
		}
		if len(raw.Receivers) == 0 {
			return nil, fmt.Errorf("pipeline %q has no receivers", name)
		}
		if len(raw.Exporters) == 0 {
			return nil, fmt.Errorf("pipeline %q has no exporters", name)
		}
		if err := validateReferences(name, "receiver", raw.Receivers, document.Receivers, usedReceivers); err != nil {
			return nil, err
		}
		if err := validateReferences(name, "processor", raw.Processors, document.Processors, usedProcessors); err != nil {
			return nil, err
		}
		if err := validateReferences(name, "exporter", raw.Exporters, document.Exporters, usedExporters); err != nil {
			return nil, err
		}
		model.Pipelines = append(model.Pipelines, Pipeline{Name: name, Signal: signal,
			Receivers: append([]string(nil), raw.Receivers...), Processors: append([]string(nil), raw.Processors...),
			Exporters: append([]string(nil), raw.Exporters...)})
	}

	for _, name := range document.Service.Extensions {
		if _, ok := document.Extensions[name]; !ok {
			return nil, fmt.Errorf("service references undefined extension %q", name)
		}
		usedExtensions[name] = true
		model.Extensions = append(model.Extensions, name)
	}

	model.Warnings = append(model.Warnings, unusedWarnings("receiver", document.Receivers, usedReceivers)...)
	model.Warnings = append(model.Warnings, unusedWarnings("processor", document.Processors, usedProcessors)...)
	model.Warnings = append(model.Warnings, unusedWarnings("exporter", document.Exporters, usedExporters)...)
	model.Warnings = append(model.Warnings, unusedWarnings("extension", document.Extensions, usedExtensions)...)
	return model, nil
}

func supportedSignal(signal string) bool {
	switch signal {
	case "metrics", "logs", "traces", "profiles":
		return true
	default:
		return false
	}
}

func validateReferences(pipelineName, kind string, references []string, defined map[string]yaml.Node, used map[string]bool) error {
	for _, name := range references {
		if _, ok := defined[name]; !ok {
			return fmt.Errorf("pipeline %q references undefined %s %q", pipelineName, kind, name)
		}
		used[name] = true
	}
	return nil
}

func unusedWarnings(kind string, defined map[string]yaml.Node, used map[string]bool) []string {
	var warnings []string
	for _, name := range sortedKeys(defined) {
		if !used[name] {
			warnings = append(warnings, fmt.Sprintf("%s %q is defined but unused", kind, name))
		}
	}
	return warnings
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
