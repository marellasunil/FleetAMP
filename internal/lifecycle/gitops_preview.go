package lifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/integrations"
)

type GitOpsFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type GitOpsPreview struct {
	ID             string       `json:"id"`
	PlanID         string       `json:"plan_id"`
	PlanHash       string       `json:"plan_hash"`
	Connection     GitConnectionSnapshot `json:"connection"`
	RepositoryPath string       `json:"repository_path"`
	Files          []GitOpsFile `json:"files"`
	Diff           string       `json:"diff"`
	PreviewHash    string       `json:"preview_hash"`
	PreparedBy     string       `json:"prepared_by"`
	CreatedAt      time.Time    `json:"created_at"`
}

// GitConnectionSnapshot freezes the repository boundary used to render a
// preview. Credentials are intentionally excluded from immutable evidence.
type GitConnectionSnapshot struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	BaseURL      string `json:"base_url,omitempty"`
	Organization string `json:"organization"`
	Project      string `json:"project,omitempty"`
	Repository   string `json:"repository"`
	Branch       string `json:"branch"`
	AllowedRoot  string `json:"allowed_root"`
	Mode         string `json:"mode"`
}

func PrepareGitOpsPreview(plan *ExecutionPlan, request *Request, connection *integrations.Connection, preparedBy string) (*GitOpsPreview, error) {
	if plan == nil || request == nil {
		return nil, errors.New("execution plan and proposal are required")
	}
	if connection == nil {
		return nil, errors.New("Git connection is required")
	}
	if plan.ExecutorKind != ExecutorGitOps {
		return nil, errors.New("execution plan is not GitOps")
	}
	if plan.RequestID != request.ID || plan.RequestSpecHash != request.SpecHash {
		return nil, errors.New("execution plan does not match immutable proposal")
	}
	if !connection.Enabled {
		return nil, errors.New("Git connection is disabled")
	}
	allowed := false
	for _, groupID := range connection.GroupIDs {
		if groupID == request.Spec.GroupID {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errors.New("Git connection is not permitted for proposal group")
	}
	preparedBy = strings.TrimSpace(preparedBy)
	if preparedBy == "" {
		return nil, errors.New("preparer is required")
	}
	payload := struct {
		APIVersion string           `json:"apiVersion"`
		Kind       string           `json:"kind"`
		PlanHash   string           `json:"planHash"`
		RequestID  string           `json:"requestId"`
		Spec       Spec             `json:"spec"`
		Targets    []TargetSnapshot `json:"targets"`
	}{"fleetamp.io/v1alpha1", "ComponentLifecycleChange", plan.PlanHash, request.ID, request.Spec, plan.Targets}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	content := string(encoded) + "\n"
	relativePath := request.Spec.GroupID + "/components/" + request.ID + ".json"
	repoPath, err := connection.ResolvePath(relativePath)
	if err != nil {
		return nil, err
	}
	lineCount := strings.Count(strings.TrimSuffix(content, "\n"), "\n") + 1
	diff := fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n+%s\n", repoPath, repoPath, repoPath, lineCount, strings.ReplaceAll(strings.TrimSuffix(content, "\n"), "\n", "\n+"))
	hashInput := struct {
		PlanHash    string                `json:"plan_hash"`
		Connection  GitConnectionSnapshot `json:"connection"`
		Path        string                `json:"path"`
		Content     string                `json:"content"`
	}{plan.PlanHash, snapshotGitConnection(connection), repoPath, content}
	canonical, _ := json.Marshal(hashInput)
	sum := sha256.Sum256(canonical)
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &GitOpsPreview{ID: hex.EncodeToString(raw), PlanID: plan.ID, PlanHash: plan.PlanHash, Connection: snapshotGitConnection(connection), RepositoryPath: repoPath, Files: []GitOpsFile{{Path: repoPath, Content: content}}, Diff: diff, PreviewHash: hex.EncodeToString(sum[:]), PreparedBy: preparedBy, CreatedAt: time.Now().UTC()}, nil
}

func snapshotGitConnection(v *integrations.Connection) GitConnectionSnapshot {
	return GitConnectionSnapshot{ID: v.ID, Name: v.Name, Provider: string(v.Provider), BaseURL: v.BaseURL, Organization: v.Organization, Project: v.Project, Repository: v.Repository, Branch: v.Branch, AllowedRoot: v.AllowedRoot, Mode: v.Mode}
}

func CloneGitOpsPreview(v *GitOpsPreview) *GitOpsPreview {
	if v == nil {
		return nil
	}
	c := *v
	c.Files = append([]GitOpsFile(nil), v.Files...)
	return &c
}
