package gitops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type fixedSecretResolver struct{ value string }

func (r fixedSecretResolver) Resolve(_ context.Context, _ string) (string, error) {
	return r.value, nil
}

func TestGitHubAdapterCreatesApprovedPullRequest(t *testing.T) {
	const token = "test-token-that-must-not-leak"
	seen := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization header missing")
		}
		seen = append(seen, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/telemetry/git/ref/heads/main":
			_, _ = w.Write([]byte(`{"object":{"sha":"base-sha"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/telemetry/git/ref/heads/fleetamp/execution":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/telemetry/git/refs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/telemetry/contents/fleetamp/groups/payments/change.json":
			if r.URL.Query().Get("ref") != "fleetamp/execution" {
				t.Errorf("unexpected content ref %q", r.URL.Query().Get("ref"))
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/acme/telemetry/contents/fleetamp/groups/payments/change.json":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			decoded, _ := base64.StdEncoding.DecodeString(payload["content"])
			if string(decoded) != "approved-content\n" || payload["branch"] != "fleetamp/execution" {
				t.Errorf("unexpected file payload: %#v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"commit":{"sha":"commit-sha"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/telemetry/pulls":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/acme/telemetry/pulls":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"number":42,"html_url":"https://github.example/acme/telemetry/pull/42"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"unexpected"}`))
		}
	}))
	defer server.Close()

	connections := memory.NewIntegrationConnectionStore()
	connection, err := integrations.NewConnection(integrations.Connection{Name: "production", Provider: integrations.GitHub, BaseURL: server.URL, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: integrations.ModePullRequest, CredentialRef: "secret://github/prod", GroupIDs: []string{"payments"}, Enabled: true}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := connections.Create(t.Context(), connection); err != nil {
		t.Fatal(err)
	}
	file := lifecycle.GitOpsFile{Path: "fleetamp/groups/payments/change.json", Content: "approved-content\n"}
	preview := &lifecycle.GitOpsPreview{ID: "preview", PreviewHash: "preview-hash", RepositoryPath: file.Path, Connection: lifecycle.GitConnectionSnapshot{ID: connection.ID, Name: connection.Name, Provider: string(connection.Provider), BaseURL: connection.BaseURL, Organization: connection.Organization, Repository: connection.Repository, Branch: connection.Branch, AllowedRoot: connection.AllowedRoot, Mode: connection.Mode}, Files: []lifecycle.GitOpsFile{file}, CreatedAt: time.Now().UTC()}
	execution := &lifecycle.GitOpsExecutionRequest{ID: "execution", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, ConnectionID: connection.ID, Provider: "github", Mode: integrations.ModePullRequest, RepositoryPath: file.Path, Branch: "main"}
	adapter := &GitHubAdapter{Connections: connections, Secrets: fixedSecretResolver{value: token}, Client: server.Client()}
	result, err := adapter.Execute(t.Context(), execution, preview)
	if err != nil {
		t.Fatal(err)
	}
	if result.Evidence["pull_request_number"] != "42" || result.Evidence["commit_sha"] != "commit-sha" || result.Evidence["repository_write"] != "true" {
		t.Fatalf("unexpected evidence: %#v", result.Evidence)
	}
	if strings.Contains(result.Message, token) || strings.Contains(strings.Join(seen, " "), token) {
		t.Fatal("credential leaked into result or request path")
	}
}

func TestEnvironmentSecretResolverMapsReferenceWithoutExposingValue(t *testing.T) {
	resolver := EnvironmentSecretResolver{Lookup: func(name string) (string, bool) {
		if name != "FLEETAMP_GITOPS_SECRET_GITHUB_PROD" {
			t.Fatalf("environment name=%q", name)
		}
		return "token", true
	}}
	value, err := resolver.Resolve(t.Context(), "secret://github/prod")
	if err != nil || value != "token" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}
