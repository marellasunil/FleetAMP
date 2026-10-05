package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestMigrationValidateStageRendersResultAndPreviewTransition(t *testing.T) {
	var output bytes.Buffer
	view := migrationView{
		Tab:             "validate",
		SelectedGroup:   "group-1",
		SelectedAgent:   "agent-1",
		SelectedPattern: "custom",
		Name:            "Imported Collector",
		Version:         "import-1",
		ProposedContent: "receivers: {}\nservice:\n  pipelines: {}\n",
		Validated:       true,
		Validation: configs.ValidationResult{
			Valid:            true,
			YAMLValid:        true,
			CollectorSkipped: true,
		},
	}
	if err := migrationPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{"Validate migration candidate", "Continue to final Preview", "proposed_yaml", "Collector distribution"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Validate page missing %q", expected)
		}
	}
}

func TestMigrationPreviewIsReadOnlyAndShowsFingerprint(t *testing.T) {
	var output bytes.Buffer
	view := migrationView{
		Tab:             "preview",
		SelectedGroup:   "group-1",
		SelectedPattern: "custom",
		Name:            "Imported Collector",
		Version:         "import-1",
		ProposedContent: "receivers: {}\nservice:\n  pipelines: {}\n",
		PreviewReady:    true,
		FinalHash:       "abc123",
	}
	if err := migrationPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{"Final preview before saving", "SHA-256 · abc123", "Save version · Next stage", "Saving remains disabled"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Preview page missing %q", expected)
		}
	}
	if strings.Contains(rendered, `action="save"`) {
		t.Fatal("Preview unexpectedly exposed a save action")
	}
}

func TestMigrationPatternAdoptionContinuesToValidateWithProposedYAML(t *testing.T) {
	if !strings.Contains(migrationHTML, `formaction="/migration?tab=validate"`) {
		t.Fatal("confirmed Pattern adoption cannot continue to Validate")
	}
	if !strings.Contains(migrationHTML, `<textarea name="proposed_yaml" hidden>{{.ProposedContent}}</textarea>`) {
		t.Fatal("Validate transition does not carry the proposed governed YAML")
	}
	if strings.Contains(migrationHTML, `Validate <span class="soon">Next</span>`) || strings.Contains(migrationHTML, `Preview <span class="soon">Planned</span>`) {
		t.Fatal("completed migration stages are still marked as upcoming")
	}
}
