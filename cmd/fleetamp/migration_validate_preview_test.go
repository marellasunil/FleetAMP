package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/migrations"
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

func TestMigrationHistoryShowsOnlyPersistedProvenance(t *testing.T) {
	var output bytes.Buffer
	view := migrationView{Tab: "history", History: []*migrations.Record{{
		ID: "record-1", ConfigurationID: "config-1", GroupID: "group-1", GroupName: "Payments",
		Name: "Imported Collector", Version: "migration-1", Source: "existing-collector",
		AgentUID: "collector-1", PatternID: "custom", ContentHash: "abc123", CreatedBy: "owner",
		CreatedAt: time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC),
	}}}
	if err := migrationPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	rendered := output.String()
	for _, expected := range []string{"Migration History", "Payments", "migration-1", "collector-1", "abc123", "owner"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("History page missing %q", expected)
		}
	}
	if strings.Contains(rendered, `History <span class="soon">Planned</span>`) {
		t.Fatal("History is still marked as planned")
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
	for _, expected := range []string{"Final preview before saving", "SHA-256 · abc123", "Save immutable version", "does not assign, submit for approval, or deploy"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Preview page missing %q", expected)
		}
	}
	if strings.Contains(rendered, `action="deploy"`) || strings.Contains(rendered, `action="submit_deployment"`) {
		t.Fatal("Preview unexpectedly exposed a deployment or approval action")
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
