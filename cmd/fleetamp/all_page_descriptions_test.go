package main

import (
	"strings"
	"testing"
)

func TestAuthenticatedPagesIncludeContextualDescription(t *testing.T) {
	pages := map[string]string{
		"fleet":                        fleetHTML,
		"groups and labels":            groupsHTML,
		"collector detail":             agentDetailHTML,
		"group detail":                 groupDetailBaseHTML,
		"upcoming capability":          upcomingHTML,
		"OTel components":              runtimeProvidersHTML,
		"collector lifecycle":          deploymentsHTML,
		"ownership lifecycle":          ownershipLifecycleHTML,
		"distributions":                distributionHTML,
		"collector build plans":        distributionBuildPlanHTML,
		"artifact promotions":          artifactPromotionHTML,
		"distribution configuration":   distributionConfigureHTML,
		"approvals":                    approvalsHTML,
		"approval detail":              approvalDetailHTML,
		"component approvals":          componentApprovalsHTML,
		"Git integrations":             integrationsHTML,
		"GitOps previews":              gitOpsPreviewsHTML,
		"GitOps executions":            gitOpsExecutionsHTML,
		"configuration builder":        blueprintsHTML,
		"configuration migration":      migrationHTMLBase,
		"fleet adoption":               fleetAdoptionHTML,
		"vendor integrations":          addonsHTML,
		"users groups and roles":       usersPageHTML,
		"my profile":                   accountPageHTML,
		"configuration policies":       sectionPolicyHTML,
		"group secrets":                groupSecretsHTML,
		"collector membership":         membershipHTML,
		"configuration version detail": configurationDetailHTML,
	}

	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(page, `class="page-description"`) {
				t.Fatalf("%s page is missing the contextual description panel", name)
			}
			if !strings.Contains(page, `class="page-description-title"`) {
				t.Fatalf("%s page is missing the contextual description title", name)
			}
		})
	}
}
