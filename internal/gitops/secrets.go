package gitops

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type SecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

type EnvironmentSecretResolver struct {
	Lookup func(string) (string, bool)
}

var secretReferencePart = regexp.MustCompile(`[^A-Za-z0-9]+`)

func (r EnvironmentSecretResolver) Resolve(_ context.Context, reference string) (string, error) {
	if r.Lookup == nil {
		return "", errors.New("secret environment resolver is unavailable")
	}
	reference = strings.TrimSpace(reference)
	if !strings.HasPrefix(reference, "secret://") {
		return "", errors.New("credential reference must use secret://")
	}
	name := strings.Trim(strings.TrimPrefix(reference, "secret://"), "/")
	if name == "" || strings.Contains(name, "..") {
		return "", errors.New("credential reference is invalid")
	}
	environmentName := "FLEETAMP_GITOPS_SECRET_" + strings.ToUpper(strings.Trim(secretReferencePart.ReplaceAllString(name, "_"), "_"))
	value, ok := r.Lookup(environmentName)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("credential %q is not configured in environment variable %s", reference, environmentName)
	}
	return strings.TrimSpace(value), nil
}
