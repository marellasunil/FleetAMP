// Package lifecycle defines immutable telemetry-component lifecycle proposals.
package lifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

type Operation string

const (
	Install Operation = "install"
	Upgrade Operation = "upgrade"
	Restart Operation = "restart"
	Remove  Operation = "remove"
)

type Status string

const Proposed Status = "proposed"

type Spec struct {
	Operation        Operation     `json:"operation"`
	ComponentType    runtimes.Type `json:"component_type"`
	GroupID          string        `json:"group_id"`
	LabelSelector    string        `json:"label_selector,omitempty"`
	DeploymentMethod string        `json:"deployment_method"`
	CurrentVersion   string        `json:"current_version,omitempty"`
	DesiredVersion   string        `json:"desired_version,omitempty"`
	Reason           string        `json:"reason"`
}

type Request struct {
	ID          string    `json:"id"`
	Spec        Spec      `json:"spec"`
	SpecHash    string    `json:"spec_hash"`
	RequestedBy string    `json:"requested_by"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

var ErrInvalidSpec = errors.New("invalid component lifecycle specification")

func NewRequest(spec Spec, requestedBy string) (*Request, error) {
	spec = normalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	requestedBy = strings.TrimSpace(requestedBy)
	if requestedBy == "" {
		return nil, errors.Join(ErrInvalidSpec, errors.New("requester is required"))
	}
	rawID := make([]byte, 16)
	if _, err := rand.Read(rawID); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	return &Request{
		ID: hex.EncodeToString(rawID), Spec: spec, SpecHash: hex.EncodeToString(digest[:]),
		RequestedBy: requestedBy, Status: Proposed, CreatedAt: time.Now().UTC(),
	}, nil
}

func normalizeSpec(spec Spec) Spec {
	spec.Operation = Operation(strings.ToLower(strings.TrimSpace(string(spec.Operation))))
	spec.ComponentType = runtimes.Type(strings.ToLower(strings.TrimSpace(string(spec.ComponentType))))
	spec.GroupID = strings.TrimSpace(spec.GroupID)
	spec.LabelSelector = strings.TrimSpace(spec.LabelSelector)
	spec.DeploymentMethod = strings.ToLower(strings.TrimSpace(spec.DeploymentMethod))
	spec.CurrentVersion = strings.TrimSpace(spec.CurrentVersion)
	spec.DesiredVersion = strings.TrimSpace(spec.DesiredVersion)
	spec.Reason = strings.TrimSpace(spec.Reason)
	return spec
}

func validateSpec(spec Spec) error {
	if spec.Operation != Install && spec.Operation != Upgrade && spec.Operation != Restart && spec.Operation != Remove {
		return errors.Join(ErrInvalidSpec, errors.New("operation must be install, upgrade, restart, or remove"))
	}
	if spec.ComponentType != runtimes.OTelCollector && spec.ComponentType != runtimes.OTelCollectorKubernetes && spec.ComponentType != runtimes.OTelOperator {
		return errors.Join(ErrInvalidSpec, errors.New("component type is not in the built-in runtime catalog"))
	}
	if spec.GroupID == "" || spec.DeploymentMethod == "" || spec.Reason == "" {
		return errors.Join(ErrInvalidSpec, errors.New("target group, deployment method, and reason are required"))
	}
	switch spec.DeploymentMethod {
	case "gitops", "kubernetes-api", "systemd", "container-runtime", "manual-package":
	default:
		return errors.Join(ErrInvalidSpec, errors.New("deployment method is not supported"))
	}
	switch spec.Operation {
	case Install:
		if spec.DesiredVersion == "" || spec.CurrentVersion != "" {
			return errors.Join(ErrInvalidSpec, errors.New("install requires desired version and no current version"))
		}
	case Upgrade:
		if spec.CurrentVersion == "" || spec.DesiredVersion == "" || spec.CurrentVersion == spec.DesiredVersion {
			return errors.Join(ErrInvalidSpec, errors.New("upgrade requires different current and desired versions"))
		}
	case Restart:
		if spec.CurrentVersion == "" || spec.DesiredVersion != "" {
			return errors.Join(ErrInvalidSpec, errors.New("restart requires current version and no desired version"))
		}
	case Remove:
		if spec.CurrentVersion == "" || spec.DesiredVersion != "" {
			return errors.Join(ErrInvalidSpec, errors.New("remove requires current version and no desired version"))
		}
	}
	return nil
}

func Clone(request *Request) *Request {
	if request == nil {
		return nil
	}
	copy := *request
	return &copy
}
