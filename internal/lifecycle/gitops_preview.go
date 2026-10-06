package lifecycle

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

type GitOpsFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type GitOpsPreview struct {
	ID             string       `json:"id"`
	PlanID         string       `json:"plan_id"`
	PlanHash       string       `json:"plan_hash"`
	RepositoryPath string       `json:"repository_path"`
	Files          []GitOpsFile `json:"files"`
	Diff           string       `json:"diff"`
	PreviewHash    string       `json:"preview_hash"`
	PreparedBy     string       `json:"prepared_by"`
	CreatedAt      time.Time    `json:"created_at"`
}

func PrepareGitOpsPreview(plan *ExecutionPlan, request *Request, preparedBy string) (*GitOpsPreview, error) {
	if plan == nil || request == nil {
		return nil, errors.New("execution plan and proposal are required")
	}
	if plan.ExecutorKind != ExecutorGitOps {
		return nil, errors.New("execution plan is not GitOps")
	}
	if plan.RequestID != request.ID || plan.RequestSpecHash != request.SpecHash {
		return nil, errors.New("execution plan does not match immutable proposal")
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
	repoPath := path.Join("fleetamp", "groups", request.Spec.GroupID, "components", request.ID+".json")
	lineCount := strings.Count(strings.TrimSuffix(content, "\n"), "\n") + 1
	diff := fmt.Sprintf("diff --git a/%s b/%s\nnew file mode 100644\n--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n+%s\n", repoPath, repoPath, repoPath, lineCount, strings.ReplaceAll(strings.TrimSuffix(content, "\n"), "\n", "\n+"))
	hashInput := struct {
		PlanHash string `json:"plan_hash"`
		Path     string `json:"path"`
		Content  string `json:"content"`
	}{plan.PlanHash, repoPath, content}
	canonical, _ := json.Marshal(hashInput)
	sum := sha256.Sum256(canonical)
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &GitOpsPreview{ID: hex.EncodeToString(raw), PlanID: plan.ID, PlanHash: plan.PlanHash, RepositoryPath: repoPath, Files: []GitOpsFile{{Path: repoPath, Content: content}}, Diff: diff, PreviewHash: hex.EncodeToString(sum[:]), PreparedBy: preparedBy, CreatedAt: time.Now().UTC()}, nil
}

func CloneGitOpsPreview(v *GitOpsPreview) *GitOpsPreview {
	if v == nil {
		return nil
	}
	c := *v
	c.Files = append([]GitOpsFile(nil), v.Files...)
	return &c
}
