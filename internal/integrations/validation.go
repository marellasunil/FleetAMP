package integrations

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type ConnectionValidation struct {
	ID, ConnectionID, ValidatedBy, Message string
	Provider                               ProviderID
	Status                                 string
	Evidence                               map[string]string
	CreatedAt, ExpiresAt                   time.Time
}

func NewConnectionValidation(connection *Connection, status, actor, message string, evidence map[string]string, ttl time.Duration) (*ConnectionValidation, error) {
	if connection == nil || connection.ID == "" || strings.TrimSpace(actor) == "" {
		return nil, errors.New("connection and validation actor are required")
	}
	if status != "passed" && status != "failed" {
		return nil, errors.New("validation status must be passed or failed")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &ConnectionValidation{ID: hex.EncodeToString(raw), ConnectionID: connection.ID, Provider: connection.Provider, Status: status, ValidatedBy: strings.TrimSpace(actor), Message: strings.TrimSpace(message), Evidence: cloneEvidence(evidence), CreatedAt: now, ExpiresAt: now.Add(ttl)}, nil
}

func (v *ConnectionValidation) Usable(now time.Time) bool {
	return v != nil && v.Status == "passed" && now.Before(v.ExpiresAt)
}

func cloneEvidence(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

type SecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

type ConnectionValidator struct {
	Secrets SecretResolver
	Client  *http.Client
}

func (v *ConnectionValidator) Validate(ctx context.Context, connection *Connection) (map[string]string, error) {
	if connection == nil || !connection.Enabled {
		return nil, errors.New("enabled connection is required")
	}
	if v.Secrets == nil {
		return nil, errors.New("credential resolver is unavailable")
	}
	token, err := v.Secrets.Resolve(ctx, connection.CredentialRef)
	if err != nil {
		return nil, fmt.Errorf("resolve credential reference: %w", err)
	}
	client := v.Client
	if client == nil {
		client = http.DefaultClient
	}
	switch connection.Provider {
	case GitHub:
		return validateGitHub(ctx, client, connection, token)
	case GitLab:
		return validateGitLab(ctx, client, connection, token)
	case AzureDevOps:
		return validateAzureDevOps(ctx, client, connection, token)
	default:
		return nil, errors.New("unsupported Git provider")
	}
}

func validateGitHub(ctx context.Context, client *http.Client, c *Connection, token string) (map[string]string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	repoURL := base + "/repos/" + url.PathEscape(c.Organization) + "/" + url.PathEscape(c.Repository)
	var repo struct {
		FullName    string `json:"full_name"`
		Permissions struct {
			Pull, Push, Admin bool
		} `json:"permissions"`
	}
	if err := providerJSON(ctx, client, http.MethodGet, repoURL, map[string]string{"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"}, &repo); err != nil {
		return nil, fmt.Errorf("validate GitHub repository: %w", err)
	}
	branchURL := repoURL + "/branches/" + url.PathEscape(c.Branch)
	if err := providerJSON(ctx, client, http.MethodGet, branchURL, map[string]string{"Authorization": "Bearer " + token, "Accept": "application/vnd.github+json"}, &struct{}{}); err != nil {
		return nil, fmt.Errorf("validate GitHub branch: %w", err)
	}
	if !repo.Permissions.Pull {
		return nil, errors.New("GitHub credential does not have repository read access")
	}
	writeRequired := c.Mode != ModeObserveOnly && c.Mode != ModeGitManaged
	if writeRequired && !repo.Permissions.Push && !repo.Permissions.Admin {
		return nil, errors.New("GitHub credential does not have repository write access required by this mode")
	}
	return map[string]string{"provider": "github", "repository": repo.FullName, "branch": c.Branch, "read": "true", "write": fmt.Sprintf("%t", repo.Permissions.Push || repo.Permissions.Admin)}, nil
}

func validateGitLab(ctx context.Context, client *http.Client, c *Connection, token string) (map[string]string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://gitlab.com"
	}
	project := url.PathEscape(path.Join(c.Organization, c.Repository))
	endpoint := base + "/api/v4/projects/" + project + "/repository/branches/" + url.PathEscape(c.Branch)
	var branch struct {
		Name string `json:"name"`
	}
	if err := providerJSON(ctx, client, http.MethodGet, endpoint, map[string]string{"PRIVATE-TOKEN": token}, &branch); err != nil {
		return nil, fmt.Errorf("validate GitLab repository and branch: %w", err)
	}
	return map[string]string{"provider": "gitlab", "repository": path.Join(c.Organization, c.Repository), "branch": branch.Name, "read": "true", "write": "not_probed"}, nil
}

func validateAzureDevOps(ctx context.Context, client *http.Client, c *Connection, token string) (map[string]string, error) {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "https://dev.azure.com"
	}
	endpoint := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/refs?filter=%s&api-version=7.1", base, url.PathEscape(c.Organization), url.PathEscape(c.Project), url.PathEscape(c.Repository), url.QueryEscape("heads/"+c.Branch))
	var refs struct {
		Value []struct {
			Name string `json:"name"`
		} `json:"value"`
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+token))
	if err := providerJSON(ctx, client, http.MethodGet, endpoint, map[string]string{"Authorization": auth}, &refs); err != nil {
		return nil, fmt.Errorf("validate Azure DevOps repository and branch: %w", err)
	}
	if len(refs.Value) == 0 {
		return nil, errors.New("Azure DevOps branch was not found")
	}
	return map[string]string{"provider": "azure-devops", "repository": c.Repository, "project": c.Project, "branch": c.Branch, "read": "true", "write": "not_probed"}, nil
}

func providerJSON(ctx context.Context, client *http.Client, method, endpoint string, headers map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider returned HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return errors.New("provider returned an invalid response")
	}
	return nil
}
