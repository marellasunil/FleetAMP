package gitops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

const defaultGitHubAPI = "https://api.github.com"

type GitHubAdapter struct {
	Connections storage.IntegrationConnectionStore
	Secrets     SecretResolver
	Client      *http.Client
}

func (a *GitHubAdapter) Execute(ctx context.Context, request *lifecycle.GitOpsExecutionRequest, preview *lifecycle.GitOpsPreview) (Result, error) {
	if request == nil || preview == nil || request.Provider != string(integrations.GitHub) {
		return Result{}, errors.New("GitHub execution evidence is required")
	}
	if request.PreviewID != preview.ID || request.PreviewHash != preview.PreviewHash || request.ConnectionID != preview.Connection.ID {
		return Result{}, errors.New("execution request does not match immutable preview")
	}
	if request.Mode != integrations.ModePullRequest {
		return Result{}, fmt.Errorf("GitHub adapter supports only %q mode", integrations.ModePullRequest)
	}
	if a.Connections == nil || a.Secrets == nil {
		return Result{}, errors.New("GitHub adapter credential services are unavailable")
	}
	connection, err := a.Connections.Get(ctx, request.ConnectionID)
	if err != nil {
		return Result{}, errors.New("GitHub connection is unavailable")
	}
	if err := validateLiveGitHubConnection(connection, preview); err != nil {
		return Result{}, err
	}
	token, err := a.Secrets.Resolve(ctx, connection.CredentialRef)
	if err != nil {
		return Result{}, err
	}
	apiBase := strings.TrimRight(connection.BaseURL, "/")
	if apiBase == "" {
		apiBase = defaultGitHubAPI
	}
	if err := validateGitHubAPIBase(apiBase); err != nil {
		return Result{}, err
	}
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	github := githubClient{baseURL: apiBase, token: token, client: client}
	baseSHA, err := github.branchSHA(ctx, connection.Organization, connection.Repository, connection.Branch)
	if err != nil {
		return Result{}, err
	}
	branch := "fleetamp/" + request.ID
	if err := github.ensureBranch(ctx, connection.Organization, connection.Repository, branch, baseSHA); err != nil {
		return Result{}, err
	}
	lastCommit := baseSHA
	for _, file := range preview.Files {
		if file.Path != request.RepositoryPath && len(preview.Files) == 1 {
			return Result{}, errors.New("approved repository path does not match preview file")
		}
		cleanFile := path.Clean(strings.TrimPrefix(file.Path, "/"))
		cleanRoot := path.Clean(connection.AllowedRoot)
		if file.Path == "" || strings.HasPrefix(file.Path, "/") || (cleanFile != cleanRoot && !strings.HasPrefix(cleanFile, cleanRoot+"/")) {
			return Result{}, errors.New("approved file escapes the GitHub connection allowed root")
		}
		lastCommit, err = github.putFile(ctx, connection.Organization, connection.Repository, branch, file, request.ID)
		if err != nil {
			return Result{}, err
		}
	}
	pullRequestURL, pullRequestNumber, err := github.ensurePullRequest(ctx, connection.Organization, connection.Repository, connection.Branch, branch, request)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Message: "GitHub pull request created or reused for the approved immutable preview",
		Evidence: map[string]string{
			"provider":            "github",
			"repository":          connection.Organization + "/" + connection.Repository,
			"base_branch":         connection.Branch,
			"head_branch":         branch,
			"commit_sha":          lastCommit,
			"pull_request_number": fmt.Sprintf("%d", pullRequestNumber),
			"pull_request_url":    pullRequestURL,
			"preview_hash":        preview.PreviewHash,
			"repository_write":    "true",
		},
	}, nil
}

func validateGitHubAPIBase(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("GitHub API base URL is invalid")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	host := parsed.Hostname()
	if parsed.Scheme == "http" && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return nil
	}
	return errors.New("GitHub API base URL must use HTTPS")
}

func validateLiveGitHubConnection(connection *integrations.Connection, preview *lifecycle.GitOpsPreview) error {
	if connection == nil || !connection.Enabled {
		return errors.New("GitHub connection is disabled")
	}
	snapshot := preview.Connection
	if string(connection.Provider) != snapshot.Provider || connection.ID != snapshot.ID || connection.BaseURL != snapshot.BaseURL || connection.Organization != snapshot.Organization || connection.Repository != snapshot.Repository || connection.Branch != snapshot.Branch || connection.AllowedRoot != snapshot.AllowedRoot || connection.Mode != snapshot.Mode {
		return errors.New("live GitHub connection no longer matches approved immutable preview")
	}
	return nil
}

type githubClient struct {
	baseURL string
	token   string
	client  *http.Client
}

type githubRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

func (c githubClient) branchSHA(ctx context.Context, owner, repository, branch string) (string, error) {
	var response githubRef
	if err := c.doJSON(ctx, http.MethodGet, c.repoURL(owner, repository, "git/ref/heads/"+branch), nil, &response, http.StatusOK); err != nil {
		return "", fmt.Errorf("read GitHub base branch: %w", err)
	}
	if response.Object.SHA == "" {
		return "", errors.New("GitHub base branch response has no commit SHA")
	}
	return response.Object.SHA, nil
}

func (c githubClient) ensureBranch(ctx context.Context, owner, repository, branch, sha string) error {
	var existing githubRef
	err := c.doJSON(ctx, http.MethodGet, c.repoURL(owner, repository, "git/ref/heads/"+branch), nil, &existing, http.StatusOK, http.StatusNotFound)
	if err != nil {
		return fmt.Errorf("check GitHub FleetAMP branch: %w", err)
	}
	if existing.Object.SHA != "" {
		return nil
	}
	payload := map[string]string{"ref": "refs/heads/" + branch, "sha": sha}
	if err := c.doJSON(ctx, http.MethodPost, c.repoURL(owner, repository, "git/refs"), payload, nil, http.StatusCreated); err != nil {
		return fmt.Errorf("create GitHub FleetAMP branch: %w", err)
	}
	return nil
}

type githubContent struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

func (c githubClient) putFile(ctx context.Context, owner, repository, branch string, file lifecycle.GitOpsFile, executionID string) (string, error) {
	endpoint := c.repoURL(owner, repository, "contents/"+file.Path)
	query := url.Values{"ref": {branch}}
	var existing githubContent
	err := c.doJSON(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil, &existing, http.StatusOK, http.StatusNotFound)
	if err != nil {
		return "", fmt.Errorf("check GitHub repository file: %w", err)
	}
	normalizedExisting := strings.ReplaceAll(existing.Content, "\n", "")
	if normalizedExisting != "" {
		decoded, decodeErr := base64.StdEncoding.DecodeString(normalizedExisting)
		if decodeErr == nil && string(decoded) == file.Content {
			return existing.SHA, nil
		}
	}
	payload := map[string]string{
		"message": "FleetAMP: apply approved execution " + executionID,
		"content": base64.StdEncoding.EncodeToString([]byte(file.Content)),
		"branch":  branch,
	}
	if existing.SHA != "" {
		payload["sha"] = existing.SHA
	}
	var response struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := c.doJSON(ctx, http.MethodPut, endpoint, payload, &response, http.StatusCreated, http.StatusOK); err != nil {
		return "", fmt.Errorf("write approved GitHub repository file: %w", err)
	}
	return response.Commit.SHA, nil
}

type githubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

func (c githubClient) ensurePullRequest(ctx context.Context, owner, repository, baseBranch, headBranch string, request *lifecycle.GitOpsExecutionRequest) (string, int, error) {
	query := url.Values{"state": {"open"}, "base": {baseBranch}, "head": {owner + ":" + headBranch}}
	var existing []githubPullRequest
	if err := c.doJSON(ctx, http.MethodGet, c.repoURL(owner, repository, "pulls")+"?"+query.Encode(), nil, &existing, http.StatusOK); err != nil {
		return "", 0, fmt.Errorf("find existing GitHub pull request: %w", err)
	}
	if len(existing) > 0 {
		return existing[0].HTMLURL, existing[0].Number, nil
	}
	payload := map[string]string{
		"title": "FleetAMP approved change " + request.ID,
		"head":  headBranch,
		"base":  baseBranch,
		"body":  "Created by FleetAMP after approval of immutable preview `" + request.PreviewHash + "`.\n\nExecution: `" + request.ID + "`",
	}
	var created githubPullRequest
	if err := c.doJSON(ctx, http.MethodPost, c.repoURL(owner, repository, "pulls"), payload, &created, http.StatusCreated); err != nil {
		return "", 0, fmt.Errorf("create GitHub pull request: %w", err)
	}
	return created.HTMLURL, created.Number, nil
}

func (c githubClient) repoURL(owner, repository, suffix string) string {
	return strings.TrimRight(c.baseURL, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repository) + "/" + escapeGitHubPath(suffix)
}

func escapeGitHubPath(value string) string {
	parts := strings.Split(path.Clean("/"+value), "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			escaped = append(escaped, url.PathEscape(part))
		}
	}
	return strings.Join(escaped, "/")
}

func (c githubClient) doJSON(ctx context.Context, method, endpoint string, payload, output any, expected ...int) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	accepted := false
	for _, status := range expected {
		if response.StatusCode == status {
			accepted = true
			break
		}
	}
	limited := io.LimitReader(response.Body, 1<<20)
	if !accepted {
		var problem struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(limited).Decode(&problem)
		if problem.Message == "" {
			problem.Message = http.StatusText(response.StatusCode)
		}
		return fmt.Errorf("GitHub API returned %d: %s", response.StatusCode, problem.Message)
	}
	if output != nil && response.StatusCode != http.StatusNotFound {
		if err := json.NewDecoder(limited).Decode(output); err != nil {
			return fmt.Errorf("decode GitHub response: %w", err)
		}
	}
	return nil
}
