package main

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"gopkg.in/yaml.v3"
)

type migrationPatternMatch struct {
	ID          string
	Name        string
	Description string
	Platform    string
	Signals     []string
	Score       int
	Confidence  string
	Evidence    []string
	Recommended bool
}

func matchCollectorPatterns(content string, patterns []*blueprints.Pattern) ([]migrationPatternMatch, error) {
	if _, _, err := inspectImportedConfiguration(content); err != nil {
		return nil, err
	}
	var document map[string]any
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		return nil, fmt.Errorf("parse standardized Collector YAML: %w", err)
	}
	receivers, _ := document["receivers"].(map[string]any)
	actualSignals := collectorPipelineSignals(document)
	result := []migrationPatternMatch{}
	seen := map[string]bool{}
	for _, pattern := range patterns {
		if pattern == nil || !pattern.Enabled || seen[pattern.ID] {
			continue
		}
		seen[pattern.ID] = true
		receiverName, receiverConfig, found := receiverByType(receivers, pattern.ReceiverID)
		if !found {
			continue
		}
		signalScore, matchedSignals := patternSignalScore(actualSignals, pattern.Signals)
		configScore, err := patternReceiverConfigScore(receiverConfig, pattern.ReceiverConfig)
		if err != nil {
			return nil, fmt.Errorf("Pattern %q has invalid receiver configuration: %w", pattern.Name, err)
		}
		score := 55 + signalScore + configScore
		if score > 100 {
			score = 100
		}
		match := migrationPatternMatch{
			ID: pattern.ID, Name: pattern.Name, Description: pattern.Description,
			Platform: pattern.Platform, Signals: append([]string(nil), pattern.Signals...),
			Score: score, Confidence: patternConfidence(score),
			Evidence: []string{fmt.Sprintf("Receiver %s matches %s", receiverName, pattern.ReceiverID)},
		}
		if len(matchedSignals) > 0 {
			match.Evidence = append(match.Evidence, "Matching pipelines: "+strings.Join(matchedSignals, ", "))
		} else {
			match.Evidence = append(match.Evidence, "No matching service pipeline signal was found")
		}
		match.Evidence = append(match.Evidence, fmt.Sprintf("Receiver structure similarity: %d%%", configScore*5))
		result = append(result, match)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score == result[j].Score {
			return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
		}
		return result[i].Score > result[j].Score
	})
	if len(result) > 0 {
		result[0].Recommended = true
	}
	return result, nil
}

func receiverByType(receivers map[string]any, receiverType string) (string, any, bool) {
	names := make([]string, 0, len(receivers))
	for name := range receivers {
		if collectorComponentType(name) == receiverType {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", nil, false
	}
	sort.Strings(names)
	name := names[0]
	return name, receivers[name], true
}

func collectorPipelineSignals(document map[string]any) map[string]bool {
	result := map[string]bool{}
	service, _ := document["service"].(map[string]any)
	pipelines, _ := service["pipelines"].(map[string]any)
	for name := range pipelines {
		signal := collectorComponentType(name)
		switch signal {
		case "metrics", "traces", "logs", "profiles":
			result[signal] = true
		}
	}
	return result
}

func patternSignalScore(actual map[string]bool, expected []string) (int, []string) {
	if len(expected) == 0 {
		return 0, nil
	}
	matched := []string{}
	for _, signal := range expected {
		if actual[signal] {
			matched = append(matched, signal)
		}
	}
	sort.Strings(matched)
	return int(math.Round(25 * float64(len(matched)) / float64(len(expected)))), matched
}

func patternReceiverConfigScore(actual any, expectedYAML string) (int, error) {
	var expected any
	if strings.TrimSpace(expectedYAML) != "" {
		if err := yaml.Unmarshal([]byte(expectedYAML), &expected); err != nil {
			return 0, err
		}
	}
	actualFeatures, expectedFeatures := map[string]bool{}, map[string]bool{}
	flattenPatternFeatures("", actual, actualFeatures)
	flattenPatternFeatures("", expected, expectedFeatures)
	if len(actualFeatures) == 0 && len(expectedFeatures) == 0 {
		return 20, nil
	}
	union := map[string]bool{}
	intersection := 0
	for feature := range actualFeatures {
		union[feature] = true
	}
	for feature := range expectedFeatures {
		if actualFeatures[feature] {
			intersection++
		}
		union[feature] = true
	}
	if len(union) == 0 {
		return 0, nil
	}
	return int(math.Round(20 * float64(intersection) / float64(len(union)))), nil
}

func flattenPatternFeatures(prefix string, value any, result map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		if prefix != "" {
			result[prefix] = true
		}
		if len(typed) == 0 && prefix != "" {
			result[prefix+"={}"] = true
		}
		for key, child := range typed {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			flattenPatternFeatures(path, child, result)
		}
	case []any:
		if prefix != "" {
			result[prefix] = true
		}
		for _, child := range typed {
			flattenPatternFeatures(prefix+"[]", child, result)
		}
	case nil:
		if prefix != "" {
			result[prefix+"=null"] = true
		}
	default:
		result[fmt.Sprintf("%s=%v", prefix, typed)] = true
	}
}

func patternConfidence(score int) string {
	switch {
	case score >= 80:
		return "High"
	case score >= 60:
		return "Medium"
	default:
		return "Low"
	}
}

func validPatternSelection(selected string, matches []migrationPatternMatch) bool {
	if selected == "custom" {
		return true
	}
	for _, match := range matches {
		if match.ID == selected {
			return true
		}
	}
	return false
}
