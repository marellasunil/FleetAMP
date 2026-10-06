package integrations

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path"
	"strings"
	"time"
)

type Connection struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Provider      ProviderID `json:"provider"`
	BaseURL       string     `json:"base_url,omitempty"`
	Organization  string     `json:"organization"`
	Project       string     `json:"project,omitempty"`
	Repository    string     `json:"repository"`
	Branch        string     `json:"branch"`
	AllowedRoot   string     `json:"allowed_root"`
	Mode          string     `json:"mode"`
	CredentialRef string     `json:"credential_ref"`
	GroupIDs      []string   `json:"group_ids"`
	Enabled       bool       `json:"enabled"`
	CreatedBy     string     `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
}

func NewConnection(v Connection, createdBy string) (*Connection, error) {
	rawRoot := strings.TrimSpace(v.AllowedRoot)
	v.Name = strings.TrimSpace(v.Name)
	v.Provider = ProviderID(strings.ToLower(strings.TrimSpace(string(v.Provider))))
	v.BaseURL = strings.TrimRight(strings.TrimSpace(v.BaseURL), "/")
	v.Organization = strings.TrimSpace(v.Organization)
	v.Project = strings.TrimSpace(v.Project)
	v.Repository = strings.TrimSpace(v.Repository)
	v.Branch = strings.TrimSpace(v.Branch)
	v.AllowedRoot = strings.Trim(rawRoot, "/")
	v.Mode = strings.TrimSpace(v.Mode)
	v.CredentialRef = strings.TrimSpace(v.CredentialRef)
	createdBy = strings.TrimSpace(createdBy)
	if v.Name == "" || v.Organization == "" || v.Repository == "" || v.Branch == "" || v.AllowedRoot == "" || v.CredentialRef == "" || createdBy == "" {
		return nil, errors.New("name, organization, repository, branch, allowed root, credential reference, and creator are required")
	}
	if v.Provider != GitHub && v.Provider != GitLab && v.Provider != AzureDevOps {
		return nil, errors.New("unsupported Git provider")
	}
	if v.Provider == AzureDevOps && v.Project == "" {
		return nil, errors.New("Azure DevOps project is required")
	}
	switch v.Mode {
	case ModeGitManaged, ModePullRequest, ModeDirectCommit, ModePrimarySync, ModeObserveOnly:
	default:
		return nil, errors.New("unsupported synchronization mode")
	}
	if !strings.HasPrefix(v.CredentialRef, "secret://") {
		return nil, errors.New("credential reference must use secret:// and must not contain a raw token")
	}
	clean := path.Clean(v.AllowedRoot)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(rawRoot, "/") {
		return nil, errors.New("allowed root must be a relative repository path")
	}
	seen := map[string]bool{}
	groups := []string{}
	for _, id := range v.GroupIDs {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			groups = append(groups, id)
		}
	}
	if len(groups) == 0 {
		return nil, errors.New("at least one FleetAMP group binding is required")
	}
	v.GroupIDs = groups
	raw := make([]byte, 16)
	if _, e := rand.Read(raw); e != nil {
		return nil, e
	}
	v.ID = hex.EncodeToString(raw)
	v.CreatedBy = createdBy
	v.CreatedAt = time.Now().UTC()
	return &v, nil
}

func (v *Connection) ResolvePath(relative string) (string, error) {
	raw := strings.TrimSpace(relative)
	relative = strings.Trim(raw, "/")
	clean := path.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(raw, "/") {
		return "", errors.New("repository path escapes allowed root")
	}
	return path.Join(v.AllowedRoot, clean), nil
}
func CloneConnection(v *Connection) *Connection {
	if v == nil {
		return nil
	}
	c := *v
	c.GroupIDs = append([]string(nil), v.GroupIDs...)
	return &c
}
