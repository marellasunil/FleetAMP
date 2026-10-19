package distributions

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ContainerOCBRunner struct {
	Runtime, BuilderImage, WorkspaceRoot, ArtifactRoot string
	MemoryLimit, CPULimit                              string
	PIDsLimit, MaxArtifactBytes                        int64
}

func (r *ContainerOCBRunner) Validate() error {
	if r.Runtime != "docker" && r.Runtime != "podman" {
		return errors.New("sandbox runtime must be docker or podman")
	}
	if !validDigestReference(r.BuilderImage) {
		return errors.New("builder image must be pinned by SHA-256 digest")
	}
	if strings.TrimSpace(r.WorkspaceRoot) == "" || strings.TrimSpace(r.ArtifactRoot) == "" {
		return errors.New("sandbox workspace and artifact roots are required")
	}
	if _, err := exec.LookPath(r.Runtime); err != nil {
		return fmt.Errorf("sandbox runtime %q is unavailable: %w", r.Runtime, err)
	}
	return nil
}

func (r *ContainerOCBRunner) Build(ctx context.Context, inspection *BuildSourceInspection) (*OCBRunResult, error) {
	if inspection == nil || inspection.Status != BuildSourceInspected {
		return nil, errors.New("an inspected source is required")
	}
	manifestSum := sha256.Sum256([]byte(inspection.ManifestYAML))
	if hex.EncodeToString(manifestSum[:]) != inspection.ManifestSHA256 {
		return nil, errors.New("OCB manifest content does not match immutable inspection evidence")
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if r.MemoryLimit == "" {
		r.MemoryLimit = "1g"
	}
	if r.CPULimit == "" {
		r.CPULimit = "1"
	}
	if r.PIDsLimit <= 0 {
		r.PIDsLimit = 256
	}
	if r.MaxArtifactBytes <= 0 {
		r.MaxArtifactBytes = 512 << 20
	}
	if err := os.MkdirAll(r.WorkspaceRoot, 0o750); err != nil {
		return nil, err
	}
	workspace, err := os.MkdirTemp(r.WorkspaceRoot, "ocb-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)
	if err := os.WriteFile(filepath.Join(workspace, "builder-config.yaml"), []byte(inspection.ManifestYAML), 0o600); err != nil {
		return nil, err
	}
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	args := []string{"run", "--rm", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=" + strconv.FormatInt(r.PIDsLimit, 10), "--memory=" + r.MemoryLimit, "--cpus=" + r.CPULimit, "--user=" + uid + ":" + gid, "--tmpfs", "/tmp:rw,noexec,nosuid,size=256m", "--env", "HOME=/tmp", "--env", "GOCACHE=/tmp/go-build", "--env", "GOMODCACHE=/opt/fleetamp/gomodcache", "--env", "GOPROXY=off", "--env", "GOSUMDB=off", "--mount", "type=bind,src=" + workspace + ",dst=/workspace,rw", "--workdir", "/workspace", r.BuilderImage, "--config", "/workspace/builder-config.yaml"}
	cmd := exec.CommandContext(ctx, r.Runtime, args...)
	output := &limitedBuffer{Limit: 64 << 10}
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("sandboxed OCB build failed: %w: %s", err, truncateOutput(output, 4096))
	}
	workspaceRoot, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, err
	}
	defer workspaceRoot.Close()
	info, err := workspaceRoot.Lstat("_build/fleetamp-collector")
	if err != nil {
		return nil, errors.New("sandboxed OCB build did not produce the expected artifact")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > r.MaxArtifactBytes {
		return nil, errors.New("sandboxed OCB artifact is not a bounded regular file")
	}
	f, err := workspaceRoot.Open("_build/fleetamp-collector")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, r.MaxArtifactBytes+1)); err != nil {
		return nil, err
	}
	sha := hex.EncodeToString(h.Sum(nil))
	if err := os.MkdirAll(r.ArtifactRoot, 0o750); err != nil {
		return nil, err
	}
	targetDir := filepath.Join(r.ArtifactRoot, sha)
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return nil, err
	}
	target := filepath.Join(targetDir, "fleetamp-collector")
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		tmp, err := os.CreateTemp(targetDir, ".artifact-")
		if err != nil {
			return nil, err
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if err := tmp.Chmod(0o500); err != nil {
			tmp.Close()
			return nil, err
		}
		written, copyErr := io.Copy(tmp, io.LimitReader(f, r.MaxArtifactBytes+1))
		closeErr := tmp.Close()
		if copyErr != nil {
			return nil, copyErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if written != info.Size() {
			return nil, errors.New("artifact size changed while copying")
		}
		if err := os.Rename(tmpName, target); err != nil {
			return nil, err
		}
	}
	return &OCBRunResult{Runtime: r.Runtime, BuilderImage: r.BuilderImage, ArtifactReference: "artifact-cache://" + sha + "/fleetamp-collector", ArtifactSHA256: sha, ArtifactSize: info.Size()}, nil
}

type limitedBuffer struct {
	bytes.Buffer
	Limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.Limit - b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.Buffer.Write(p)
	}
	return original, nil
}
func truncateOutput(value *limitedBuffer, limit int) string {
	text := strings.TrimSpace(value.String())
	if len(text) > limit {
		return text[:limit] + "…"
	}
	return text
}
