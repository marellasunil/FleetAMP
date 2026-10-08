package gitops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

type Result struct {
	Message  string
	Evidence map[string]string
}

type StatusResult struct {
	Status   lifecycle.GitOpsExecutionStatus
	Message  string
	Evidence map[string]string
}

// Adapter is the provider boundary. A future GitHub, GitLab, or Azure DevOps
// implementation can satisfy this interface without changing lifecycle code.
type Adapter interface {
	Execute(context.Context, *lifecycle.GitOpsExecutionRequest, *lifecycle.GitOpsPreview) (Result, error)
}

type StatusAdapter interface {
	Status(context.Context, *lifecycle.GitOpsExecutionRequest, *lifecycle.GitOpsPreview, map[string]string) (StatusResult, error)
}

type Registry struct{ adapters map[string]Adapter }

func NewDryRunRegistry() *Registry {
	noop := NoopAdapter{}
	return &Registry{adapters: map[string]Adapter{"github": noop, "gitlab": noop, "azure-devops": noop}}
}

func (r *Registry) StatusAdapter(provider string) (StatusAdapter, error) {
	adapter, err := r.Adapter(provider)
	if err != nil {
		return nil, err
	}
	statusAdapter, ok := adapter.(StatusAdapter)
	if !ok {
		return nil, fmt.Errorf("Git provider %q does not support status synchronization", provider)
	}
	return statusAdapter, nil
}

func (r *Registry) Adapter(provider string) (Adapter, error) {
	adapter, ok := r.adapters[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported Git provider %q", provider)
	}
	return adapter, nil
}

func (r *Registry) Register(provider string, adapter Adapter) error {
	if r == nil || strings.TrimSpace(provider) == "" || adapter == nil {
		return errors.New("provider and adapter are required")
	}
	if r.adapters == nil {
		r.adapters = map[string]Adapter{}
	}
	r.adapters[strings.TrimSpace(provider)] = adapter
	return nil
}

type NoopAdapter struct{}

func (NoopAdapter) Execute(_ context.Context, request *lifecycle.GitOpsExecutionRequest, preview *lifecycle.GitOpsPreview) (Result, error) {
	if request == nil || preview == nil || request.PreviewID != preview.ID || request.PreviewHash != preview.PreviewHash || request.ConnectionID != preview.Connection.ID {
		return Result{}, errors.New("execution request does not match immutable preview")
	}
	return Result{Message: "Dry-run provider adapter completed; no repository write was performed", Evidence: map[string]string{"dry_run": "true", "provider": request.Provider, "mode": request.Mode, "branch": request.Branch, "repository_path": request.RepositoryPath, "file_count": fmt.Sprintf("%d", len(preview.Files))}}, nil
}
