package runtimes

import "testing"

func TestDefaultInstallationGuidesAreCompleteAndDefensive(t *testing.T) {
	guides := DefaultInstallationGuides()
	if len(guides) != 5 {
		t.Fatalf("DefaultInstallationGuides() returned %d guides, want 5", len(guides))
	}
	seen := map[string]bool{}
	for _, guide := range guides {
		if guide.ID == "" || guide.Name == "" || guide.RuntimeType == "" || guide.DeploymentMethod == "" || guide.Owner == "" || len(guide.Prerequisites) == 0 || len(guide.Steps) == 0 || len(guide.Verification) == 0 {
			t.Fatalf("incomplete installation guide: %#v", guide)
		}
		if seen[guide.ID] {
			t.Fatalf("duplicate installation guide ID %q", guide.ID)
		}
		seen[guide.ID] = true
	}

	guides[0].Prerequisites[0] = "changed"
	if DefaultInstallationGuides()[0].Prerequisites[0] == "changed" {
		t.Fatal("DefaultInstallationGuides() exposed mutable catalog state")
	}
}
