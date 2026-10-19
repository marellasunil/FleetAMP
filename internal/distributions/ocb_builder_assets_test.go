package distributions

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOfflineOCBBuilderDockerfileKeepsSupplyChainControls(t *testing.T) {
	raw, err := os.ReadFile("../../build/ocb-builder/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"ARG GO_IMAGE",
		"FROM ${GO_IMAGE}",
		"go run ./cmd/ocb-cache-manifest",
		"go mod download all",
		"GOPROXY=off GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"CGO_ENABLED=0",
		"/go/bin/builder --config /tmp/smoke-builder.yaml",
		"chmod -R a-w \"${GOMODCACHE}\"",
		"USER 65532:65532",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("offline builder Dockerfile lost required control %q", required)
		}
	}
}

func TestOfflineOCBBuilderWorkflowIsManualAndAttested(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/ocb-builder-image.yml")
	if err != nil {
		t.Fatal(err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(raw, &document); err != nil {
		t.Fatalf("invalid workflow YAML: %v", err)
	}
	text := string(raw)
	for _, required := range []string{
		"workflow_dispatch:",
		"^docker\\.io/library/golang:1\\.26\\.0-bookworm@sha256:",
		"actions/attest-build-provenance@v3",
		"provenance: mode=max",
		"sbom: true",
		"packages: write",
		"attestations: write",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("offline builder workflow lost required control %q", required)
		}
	}
	for _, forbidden := range []string{"\n  push:", "\n  pull_request:"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("offline builder publication workflow must remain manual-only: found %q", forbidden)
		}
	}
}
