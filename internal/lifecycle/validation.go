package lifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/agents"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

type ValidationStatus string

const (
	ValidationCompatible ValidationStatus = "compatible"
	ValidationAttention  ValidationStatus = "attention"
	ValidationBlocked    ValidationStatus = "blocked"
)

type Finding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	TargetID string `json:"target_id,omitempty"`
	Message  string `json:"message"`
}

type TargetSnapshot struct {
	InstanceUID string             `json:"instance_uid"`
	Name        string             `json:"name"`
	Type        agents.AgentType   `json:"type"`
	Version     string             `json:"version,omitempty"`
	Runtime     agents.RuntimeType `json:"runtime"`
	Cluster     string             `json:"cluster,omitempty"`
	Namespace   string             `json:"namespace,omitempty"`
	Connected   bool               `json:"connected"`
	Healthy     bool               `json:"healthy"`
	Labels      map[string]string  `json:"labels,omitempty"`
}

type Validation struct {
	ID             string           `json:"id"`
	RequestID      string           `json:"request_id"`
	RequestSpecHash string          `json:"request_spec_hash"`
	GroupID        string           `json:"group_id"`
	GroupName      string           `json:"group_name"`
	GroupSelector  map[string]string `json:"group_selector"`
	LabelSelector  map[string]string `json:"label_selector,omitempty"`
	Targets        []TargetSnapshot `json:"targets"`
	Findings       []Finding        `json:"findings"`
	Status         ValidationStatus `json:"status"`
	ResultHash     string           `json:"result_hash"`
	ValidatedBy    string           `json:"validated_by"`
	CreatedAt      time.Time        `json:"created_at"`
}

var ErrInvalidSelector = errors.New("invalid label selector")

func ParseLabelSelector(value string) (map[string]string, error) {
	result := map[string]string{}
	value = strings.TrimSpace(value)
	if value == "" {
		return result, nil
	}
	for _, expression := range strings.Split(value, ",") {
		parts := strings.Split(expression, "=")
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("%w: use comma-separated key=value expressions", ErrInvalidSelector)
		}
		key, wanted := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("%w: duplicate key %q", ErrInvalidSelector, key)
		}
		result[key] = wanted
	}
	return result, nil
}

func ValidateRequest(request *Request, group *groups.Group, inventory []*agents.ManagedAgent, validatedBy string) (*Validation, error) {
	if request == nil || group == nil || request.Spec.GroupID != group.ID {
		return nil, errors.New("lifecycle request and target group are required")
	}
	labels, err := ParseLabelSelector(request.Spec.LabelSelector)
	if err != nil {
		return nil, err
	}
	validation := &Validation{RequestID: request.ID, RequestSpecHash: request.SpecHash, GroupID: group.ID, GroupName: group.Name,
		GroupSelector: cloneStrings(group.Selector), LabelSelector: labels, ValidatedBy: strings.TrimSpace(validatedBy), CreatedAt: time.Now().UTC()}
	if validation.ValidatedBy == "" {
		return nil, errors.New("validator is required")
	}
	if !group.Enabled {
		validation.Findings = append(validation.Findings, Finding{Code: "group_disabled", Severity: "blocking", Message: "Target group is disabled."})
	}
	for _, agent := range inventory {
		if !groups.MatchesIdentity(group, agent) || !matchesLabels(groups.EffectiveLabels(agent), labels) {
			continue
		}
		target := TargetSnapshot{InstanceUID: agent.InstanceUID, Name: agent.Name, Type: agent.Type, Version: agent.Version,
			Runtime: agent.Deployment.Runtime, Cluster: agent.Deployment.Cluster, Namespace: agent.Deployment.Namespace,
			Connected: agent.Connected, Healthy: agent.Healthy, Labels: cloneStrings(groups.EffectiveLabels(agent))}
		validation.Targets = append(validation.Targets, target)
		validation.Findings = append(validation.Findings, validateTarget(request.Spec, agent)...)
	}
	sort.Slice(validation.Targets, func(i, j int) bool { return validation.Targets[i].InstanceUID < validation.Targets[j].InstanceUID })
	sort.SliceStable(validation.Findings, func(i, j int) bool {
		if validation.Findings[i].TargetID != validation.Findings[j].TargetID { return validation.Findings[i].TargetID < validation.Findings[j].TargetID }
		return validation.Findings[i].Code < validation.Findings[j].Code
	})
	if len(validation.Targets) == 0 {
		validation.Findings = append(validation.Findings, Finding{Code: "no_targets", Severity: "blocking", Message: "No managed components match the group and label selectors."})
	}
	validation.Status = ValidationCompatible
	for _, finding := range validation.Findings {
		if finding.Severity == "blocking" { validation.Status = ValidationBlocked; break }
		if validation.Status == ValidationCompatible { validation.Status = ValidationAttention }
	}
	rawID := make([]byte, 16)
	if _, err := rand.Read(rawID); err != nil { return nil, err }
	validation.ID = hex.EncodeToString(rawID)
	hashInput := struct {
		RequestID string `json:"request_id"`; RequestSpecHash string `json:"request_spec_hash"`; GroupID string `json:"group_id"`
		GroupName string `json:"group_name"`; GroupSelector map[string]string `json:"group_selector"`; LabelSelector map[string]string `json:"label_selector"`
		Targets []TargetSnapshot `json:"targets"`; Findings []Finding `json:"findings"`; Status ValidationStatus `json:"status"`
	}{validation.RequestID, validation.RequestSpecHash, validation.GroupID, validation.GroupName, validation.GroupSelector, validation.LabelSelector, validation.Targets, validation.Findings, validation.Status}
	encoded, err := json.Marshal(hashInput)
	if err != nil { return nil, err }
	digest := sha256.Sum256(encoded)
	validation.ResultHash = hex.EncodeToString(digest[:])
	return validation, nil
}

func validateTarget(spec Spec, agent *agents.ManagedAgent) []Finding {
	findings := []Finding{}
	add := func(code, severity, message string) { findings = append(findings, Finding{Code: code, Severity: severity, TargetID: agent.InstanceUID, Message: message}) }
	if agent.Type != agents.AgentTypeOTelCollector { add("component_type", "blocking", "Target is not a built-in OpenTelemetry Collector component.") }
	wantsKubernetes := spec.ComponentType == runtimes.OTelCollectorKubernetes || spec.ComponentType == runtimes.OTelOperator
	if wantsKubernetes && agent.Deployment.Runtime != agents.RuntimeKubernetes { add("runtime_component_mismatch", "blocking", "Kubernetes component requests require Kubernetes targets.") }
	if spec.ComponentType == runtimes.OTelCollector && agent.Deployment.Runtime == agents.RuntimeKubernetes { add("runtime_component_mismatch", "blocking", "Use the Kubernetes Collector component type for Kubernetes targets.") }
	if spec.ComponentType == runtimes.OTelOperator { add("operator_inventory", "blocking", "Operator inventory is not yet reported through the built-in management provider.") }
	switch spec.DeploymentMethod {
	case "gitops", "kubernetes-api":
		if agent.Deployment.Runtime != agents.RuntimeKubernetes { add("delivery_runtime_mismatch", "blocking", "Selected delivery method requires a Kubernetes target.") }
	case "systemd":
		if agent.Deployment.Runtime != agents.RuntimeVM && agent.Deployment.Runtime != agents.RuntimeBareMetal { add("delivery_runtime_mismatch", "blocking", "System service delivery requires a VM or bare-metal target.") }
	case "container-runtime":
		if agent.Deployment.Runtime != agents.RuntimeContainer { add("delivery_runtime_mismatch", "blocking", "Container runtime delivery requires a container target.") }
	}
	if spec.Operation != Install && strings.TrimSpace(agent.Version) != spec.CurrentVersion { add("current_version_mismatch", "blocking", "Reported version does not match the proposal current version.") }
	if !agent.Connected { add("target_offline", "advisory", "Target is not currently connected.") }
	if !agent.Healthy { add("target_unhealthy", "advisory", "Target does not currently report healthy status.") }
	if !hasString(agent.Capabilities, "reports_health") { add("health_reporting", "advisory", "Target does not advertise health reporting required for post-change verification.") }
	return findings
}

func matchesLabels(actual, selector map[string]string) bool { for key, value := range selector { if actual[key] != value { return false } }; return true }
func cloneStrings(values map[string]string) map[string]string { result := make(map[string]string, len(values)); for key, value := range values { result[key] = value }; return result }
func hasString(values []string, wanted string) bool { for _, value := range values { if value == wanted { return true } }; return false }

func CloneValidation(validation *Validation) *Validation {
	if validation == nil { return nil }
	copy := *validation
	copy.GroupSelector = cloneStrings(validation.GroupSelector); copy.LabelSelector = cloneStrings(validation.LabelSelector)
	copy.Targets = append([]TargetSnapshot(nil), validation.Targets...); copy.Findings = append([]Finding(nil), validation.Findings...)
	for i := range copy.Targets { copy.Targets[i].Labels = cloneStrings(validation.Targets[i].Labels) }
	return &copy
}
