package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ORASArtifactPublisher uploads a preflighted collector binary as an OCI
// artifact. It is only constructed when the opt-in publication worker is
// enabled. Credentials are passed on stdin and never placed in argv or logs.
type ORASArtifactPublisher struct {
	Executable, ArtifactRoot, StagingRoot, Username string
	Secrets                                         PublicationSecretResolver
	MaxBytes                                        int64
}

func (p *ORASArtifactPublisher) Validate() error {
	if strings.TrimSpace(p.ArtifactRoot) == "" || strings.TrimSpace(p.StagingRoot) == "" {
		return errors.New("artifact and publication staging roots are required")
	}
	if p.Secrets == nil {
		return errors.New("publication credential resolver is required")
	}
	executable := strings.TrimSpace(p.Executable)
	if executable == "" {
		executable = "oras"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return fmt.Errorf("ORAS executable is unavailable: %w", err)
	}
	p.Executable = path
	if strings.TrimSpace(p.Username) == "" {
		p.Username = "oauth2"
	}
	return nil
}

func (p *ORASArtifactPublisher) Publish(ctx context.Context, preflight *ArtifactPublicationPreflight) (*RegistryPublishResult, error) {
	if preflight == nil || preflight.Status != ArtifactPublicationPreflightPassed {
		return nil, errors.New("successful publication preflight is required")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	secret, err := p.Secrets.Resolve(ctx, preflight.CredentialRef)
	if err != nil {
		return nil, errors.New("registry credential is unavailable")
	}
	defer func() { secret = "" }()
	relative, err := resolveArtifactRelativePath(preflight.ArtifactReference)
	if err != nil {
		return nil, err
	}
	staged, cleanup, err := p.stageExactArtifact(relative, preflight.ArtifactSHA256, preflight.ArtifactSize)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	target := strings.TrimPrefix(preflight.DestinationRef, "oci://")
	args := []string{"push", "--username", p.Username, "--password-stdin", target, staged + ":application/vnd.fleetamp.collector.executable"}
	cmd := exec.CommandContext(ctx, p.Executable, args...)
	cmd.Stdin = strings.NewReader(secret + "\n")
	output := &limitedBuffer{Limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ORAS registry publication failed: %w", err)
	}
	return &RegistryPublishResult{ArtifactSHA256: preflight.ArtifactSHA256, DestinationRef: preflight.DestinationRef}, nil
}

func (p *ORASArtifactPublisher) stageExactArtifact(relative, expectedSHA string, expectedSize int64) (string, func(), error) {
	root, err := os.OpenRoot(p.ArtifactRoot)
	if err != nil {
		return "", func() {}, fmt.Errorf("open artifact root: %w", err)
	}
	defer root.Close()
	source, err := root.Open(relative)
	if err != nil {
		return "", func() {}, fmt.Errorf("open cached artifact: %w", err)
	}
	defer source.Close()
	if err := os.MkdirAll(p.StagingRoot, 0o700); err != nil {
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp(p.StagingRoot, "publish-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	target := filepath.Join(dir, "fleetamp-collector")
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o400)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	h := sha256.New()
	maxBytes := p.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 512 << 20
	}
	written, copyErr := io.Copy(io.MultiWriter(f, h), io.LimitReader(source, maxBytes+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || written != expectedSize || written <= 0 || written > maxBytes || hex.EncodeToString(h.Sum(nil)) != expectedSHA {
		cleanup()
		return "", func() {}, errors.New("cached artifact changed after publication preflight")
	}
	return target, cleanup, nil
}
