// Configuration validation for FleetAMP.
//
// Purpose:
//
//	Rejects malformed configuration before it becomes desired state and,
//	when configured, asks the real OpenTelemetry Collector distribution to
//	validate component names, pipelines, and component-specific settings.
//
// Validation flow:
//
//	configuration content -> YAML syntax validation -> optional Collector
//	`validate` command -> validation result -> create/assign decision.
//
// Dependencies:
//
//	gopkg.in/yaml.v3 for syntax parsing and the configured otelcol binary for
//	distribution-aware validation. No Collector Go packages are embedded.
//
// Configuration:
//
//	FLEETAMP_OTELCOL_BINARY is read by the application and passed to the
//	validator. When unset, YAML validation remains mandatory and Collector
//	validation is reported as skipped.
package configs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ValidationResult describes all checks performed before configuration use.
type ValidationResult struct {
	Valid              bool   `json:"valid"`
	YAMLValid          bool   `json:"yaml_valid"`
	CollectorValidated bool   `json:"collector_validated"`
	CollectorSkipped   bool   `json:"collector_skipped"`
	Error              string `json:"error,omitempty"`
}

// Validator validates configuration syntax and optionally delegates semantic
// validation to a concrete OpenTelemetry Collector distribution.
type Validator struct {
	CollectorBinary string
	Timeout         time.Duration
}

// NewValidator returns a validator with a bounded external validation timeout.
func NewValidator(collectorBinary string) *Validator {
	return &Validator{CollectorBinary: strings.TrimSpace(collectorBinary), Timeout: 15 * time.Second}
}

// Validate first parses YAML locally. If CollectorBinary is configured, the
// same content is then validated using `otelcol validate --config=<tempfile>`.
func (v *Validator) Validate(ctx context.Context, content string) ValidationResult {
	result := ValidationResult{}
	if strings.TrimSpace(content) == "" {
		result.Error = "configuration content is empty"
		return result
	}

	var document any
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		result.Error = fmt.Sprintf("invalid YAML: %v", err)
		return result
	}
	result.YAMLValid = true

	if v == nil || v.CollectorBinary == "" {
		result.Valid = true
		result.CollectorSkipped = true
		return result
	}

	timeout := v.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	validateCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "fleetamp-config-validate-*")
	if err != nil {
		result.Error = fmt.Sprintf("create validation workspace: %v", err)
		return result
	}
	defer os.RemoveAll(dir)

	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		result.Error = fmt.Sprintf("write validation config: %v", err)
		return result
	}

	cmd := exec.CommandContext(validateCtx, v.CollectorBinary, "validate", "--config="+configPath)
	output, err := cmd.CombinedOutput()
	if validateCtx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("collector validation timed out after %s", timeout)
		return result
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		result.Error = "collector validation failed: " + message
		return result
	}

	result.CollectorValidated = true
	result.Valid = true
	return result
}
