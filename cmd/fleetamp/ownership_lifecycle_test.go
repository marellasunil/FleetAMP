package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func TestOwnershipLifecyclePageAggregatesGovernedChanges(t *testing.T) {
	ctx := context.Background()
	database, err := sqlitestore.Open(ctx, filepath.Join(t.TempDir(), "ownership.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	change, err := configs.NewMembershipChange(configs.MembershipChange{
		Operation: configs.MembershipMove, SourceGroupID: "payments-dev", SourceGroupName: "Payments Dev",
		TargetGroupID: "payments-prod", TargetGroupName: "Payments Prod",
		Targets: []configs.MembershipTarget{{AgentInstanceUID: "collector-1", AgentName: "payments-collector-01"}},
		RequestedBy: "operator", AssignedReviewer: "owner", Reason: "promote ownership",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.MembershipChanges().Create(ctx, change); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerOwnershipLifecycleRoutes(mux, database.MembershipChanges(), database.Groups(), nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ownership-lifecycle", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, expected := range []string{"Ownership Lifecycle", "Payments Dev", "Payments Prod", "payments-collector-01", "pending_approval"} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Errorf("ownership lifecycle page missing %q", expected)
		}
	}
}

func TestSideNavigationSeparatesCollectorAndOwnershipLifecycle(t *testing.T) {
	for _, expected := range []string{
		`<div class="navlabel">Lifecycle</div>`, `href="/deployments"`, "Collector Lifecycle",
		`href="/ownership-lifecycle"`, "Ownership Lifecycle", "Distributions", "Bundles", "Rollouts",
	} {
		if !strings.Contains(sideNav, expected) {
			t.Errorf("lifecycle navigation missing %q", expected)
		}
	}
}
