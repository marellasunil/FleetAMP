// Tests for FleetAMP per-agent deployment operational summaries.
package main

import (
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestSummarizeDeploymentsUsesLatestAppliedVersion(t *testing.T) {
	now := time.Now().UTC()
	sent := now.Add(-8 * time.Second)
	applied := now.Add(-3 * time.Second)
	failedSent := now.Add(-2 * time.Second)
	failed := now.Add(-time.Second)
	items := []*configs.Deployment{
		{ConfigurationName: "gateway.yaml", ConfigurationVersion: "8", Status: configs.DeliveryFailed, SentAt: &failedSent, FailedAt: &failed},
		{ConfigurationName: "gateway.yaml", ConfigurationVersion: "7", Status: configs.DeliveryApplied, SentAt: &sent, AppliedAt: &applied},
	}

	summary := summarizeDeployments(items)
	if summary.CurrentDeployedVersion != "7" {
		t.Fatalf("current deployed version=%q, want 7", summary.CurrentDeployedVersion)
	}
	if summary.LastDeployment != items[0] {
		t.Fatal("last deployment should be newest attempt")
	}
	if summary.LastSuccessful != items[1] {
		t.Fatal("last successful deployment should be latest applied deployment")
	}
	if summary.LastDeploymentDuration != "1s" {
		t.Fatalf("last deployment duration=%q, want 1s", summary.LastDeploymentDuration)
	}
}

func TestDeploymentDurationUnknownWhileInProgress(t *testing.T) {
	now := time.Now().UTC()
	if got := deploymentDuration(&configs.Deployment{SentAt: &now, Status: configs.DeliveryApplying}); got != "" {
		t.Fatalf("duration=%q, want empty while deployment is in progress", got)
	}
}
