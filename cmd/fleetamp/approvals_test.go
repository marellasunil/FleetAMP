package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestConfigurationLineDiffHighlightsChanges(t *testing.T) {
	rows := configurationLineDiff("receivers:\n  otlp:\nservice: {}\n", "receivers:\n  otlp:\nprocessors: {}\nservice: {}\n")
	if len(rows) != 4 {
		t.Fatalf("row count=%d, want 4: %#v", len(rows), rows)
	}
	if rows[0].Kind != "same" || rows[1].Kind != "same" || rows[2].Kind != "added" || rows[2].NewText != "processors: {}" || rows[3].Kind != "same" {
		t.Fatalf("unexpected diff: %#v", rows)
	}
}

func TestApprovalStatusMatches(t *testing.T) {
	if !approvalStatusMatches("active", configs.GroupDeploymentPendingApproval) ||
		!approvalStatusMatches("active", configs.GroupDeploymentDeploying) ||
		approvalStatusMatches("active", configs.GroupDeploymentExpired) ||
		!approvalStatusMatches("expired", configs.GroupDeploymentExpired) ||
		!approvalStatusMatches("sent_back", configs.GroupDeploymentSentBack) ||
		!approvalStatusMatches("all", configs.GroupDeploymentRejected) {
		t.Fatal("approval status filter returned an unexpected result")
	}
}

func TestApprovalQueueLinksGroupAndShowsReviewerActions(t *testing.T) {
	request := &configs.GroupDeploymentRequest{
		ID: "request-1", GroupID: "group-1", GroupName: "payments · prod · eu",
		ConfigurationName: "collector.yaml", ConfigurationVersion: "7",
		Status: configs.GroupDeploymentPendingApproval, AssignedReviewer: "reviewer-admin",
	}
	view := approvalsView{Page: "approvals", Status: "active", Items: []approvalItem{{
		Request: request, ValidationStatus: "Passed at submission", CanReview: true,
	}}}
	var output bytes.Buffer
	if err := approvalsPage.Execute(&output, view); err != nil {
		t.Fatal(err)
	}
	body := output.String()
	for _, expected := range []string{
		`href="/groups/group-1"`, `action="/groups/group-1"`,
		`value="approve_deployment"`, `value="reject_deployment"`, `value="send_back_deployment"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("approval queue does not contain %q", expected)
		}
	}
}

func TestConfigurationLineDiffMarksRemoval(t *testing.T) {
	rows := configurationLineDiff("receivers: {}\nprocessors: {}\n", "receivers: {}\n")
	if len(rows) != 2 || rows[1].Kind != "removed" || rows[1].OldText != "processors: {}" {
		t.Fatalf("unexpected diff: %#v", rows)
	}
}
