package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSourceAcquisitionRequiresVerifiedCacheEvidence(t *testing.T) {
	source := &BuildSourceResolution{ID: "source-1", ExecutionID: "execution-1", BuildPlanHash: "plan", Status: BuildSourceResolved}
	sha := strings.Repeat("a", 64)
	item, err := NewBuildSourceAcquisition(source, sha, 123, "source-cache://"+sha+".tar.gz", "worker", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != BuildSourceAcquired {
		t.Fatalf("unexpected item: %#v", item)
	}
	if _, err := NewBuildSourceAcquisition(source, "bad", 123, "source-cache://bad", "worker", time.Now()); err == nil {
		t.Fatal("expected checksum rejection")
	}
}
func TestOfficialSourceURLPolicy(t *testing.T) {
	if err := validateOfficialSourceURL("https://codeload.github.com/open-telemetry/opentelemetry-collector-releases/tar.gz/refs/tags/v0.140.0"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"http://codeload.github.com/open-telemetry/opentelemetry-collector-releases/tar.gz/refs/tags/v0.140.0", "https://example.com/archive", "https://codeload.github.com/open-telemetry/opentelemetry-collector-releases/tar.gz/refs/tags/../main"} {
		if err := validateOfficialSourceURL(raw); err == nil {
			t.Fatalf("expected rejection for %s", raw)
		}
	}
}
